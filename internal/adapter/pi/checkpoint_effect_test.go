package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

func TestCheckpointEffectReplaysOnlyItsRecordedObservation(t *testing.T) {
	root, operation, intent, spec := checkpointEffectFixture(t)
	result, err := CheckpointEffect(operation, intent, spec)
	if err != nil || result.Receipt.IntentID != intent.ID {
		t.Fatalf("checkpoint effect = %#v, %v", result, err)
	}
	reopened, _ := run.Open(root, "checkpoint-effect", "worker")
	again, err := CheckpointEffect(reopened, intent, spec)
	if err != nil || again.Receipt != result.Receipt || again.Checkpoint.Checkpoint != result.Checkpoint.Checkpoint {
		t.Fatalf("replayed checkpoint = %#v, %v", again, err)
	}
}

func TestCheckpointEffectRefusesUnknownAttempt(t *testing.T) {
	_, operation, intent, spec := checkpointEffectFixture(t)
	payload, err := operation.ReadEffectPayload(intent)
	if err != nil {
		t.Fatal(err)
	}
	var decoded CheckpointPayload
	if strictjson.Decode(payload, &decoded) != nil {
		t.Fatal("invalid test payload")
	}
	attempt, err := json.Marshal(checkpointAttempt{Contract: checkpointAttemptContract, SessionPath: spec.SessionPath, Payload: decoded})
	if err != nil {
		t.Fatal(err)
	}
	if created, err := operation.BeginEffectAttempt(intent, attempt); err != nil || !created {
		t.Fatalf("attempt = %t, %v", created, err)
	}
	if _, err := CheckpointEffect(operation, intent, spec); err == nil {
		t.Fatal("checkpoint repeated unknown attempt")
	}
}

func checkpointEffectFixture(t *testing.T) (string, *run.Operation, effect.Intent, CheckpointEffectSpec) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "agentlab")
	session := writeTree(t, []string{
		`{"type":"session","version":3,"id":"checkpoint-effect-session"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"go"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"text","text":"public"}]}}`,
	})
	bindAdapterTestManifest(t, root, "checkpoint-effect", "worker")
	operation, _ := run.Open(root, "checkpoint-effect", "worker")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := Begin(operation, session, policy, nil); err != nil {
		t.Fatal(err)
	}
	sdkRoot := installedSDKRoot(t)
	_, source, _, _ := runtime.Caller(0)
	artifactRoot := buildContextArtifact(t, source)
	binary, err := os.ReadFile(filepath.Join(artifactRoot, "bin", "agentlab"))
	if err != nil {
		t.Fatal(err)
	}
	config := IdentityConfig{SDKRoot: sdkRoot, ContextFilterPath: filepath.Join(artifactRoot, "extension.ts"), AdapterDigest: sha256Digest(binary), Provider: "test", Model: "test", ThinkingPolicy: "off", CompactionPolicy: "off"}
	identity, err := VerifyRuntimeIdentity(config)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := ReadPublicTree(session)
	if err != nil {
		t.Fatal(err)
	}
	entrySource, err := tree.Entries[1].EvidenceSource()
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := operation.AcquireAdapterWriter(adapterName)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit([]byte("checkpoint-cursor"), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: run.EvidenceAssistantMessage, SourceLocator: entrySource, Label: "assistant_message", Raw: []byte("public")}}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	evidence, err := operation.EvidenceForSourceLocator(entrySource)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCheckpointPayload(CheckpointPayload{EntryLocator: tree.Entries[1].Locator, Evidence: evidence, PrefixDigest: tree.Entries[1].PrefixDigest, Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := artifact.NewStore(filepath.Join(root, "artifacts")).Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	return root, operation, effect.Intent{ID: "checkpoint-1", RunID: "worker", Kind: effect.Checkpoint, Payload: ref}, CheckpointEffectSpec{SDKRoot: sdkRoot, ContextFilterPath: config.ContextFilterPath, SessionPath: session}
}
