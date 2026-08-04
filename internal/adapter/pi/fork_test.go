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

func TestForkReconcilesOneUnknownSDKChildWithoutRetry(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "agentlab")
	parentPath := writeTree(t, []string{
		`{"type":"session","version":3,"id":"parent"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"ALPHA"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"text","text":"ready"}]}}`,
		`{"type":"message","id":"poison","parentId":"assistant","message":{"role":"user","content":"POISON"}}`,
	})
	bindAdapterTestManifest(t, root, "fork-exp", "parent-run")
	operation, _ := run.Open(root, "fork-exp", "parent-run")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := Begin(operation, parentPath, policy, nil); err != nil {
		t.Fatal(err)
	}
	parent, err := ReadPublicTree(parentPath)
	if err != nil {
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
	checkpoint, err := Checkpoint(operation, parentPath, parent.Entries[1].Locator, identity)
	if err != nil {
		t.Fatal(err)
	}
	payloadData, err := EncodeForkPayload(ForkPayload{Checkpoint: checkpoint.Checkpoint.Checkpoint, Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := artifact.NewStore(filepath.Join(root, "artifacts")).Put(payloadData)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "fork-1", RunID: "parent-run", Kind: effect.Fork, Payload: payload}
	spec := ForkSpec{SDKRoot: sdkRoot, ContextFilterPath: config.ContextFilterPath, ParentSession: parentPath, ChildSessionDir: filepath.Join(dir, "children")}
	if err := stageUnknownFork(t, operation, intent, spec); err != nil {
		t.Fatal(err)
	}
	reopened, _ := run.Open(root, "fork-exp", "parent-run")
	result, err := Fork(reopened, intent, spec)
	if err != nil || result.Receipt.IntentID != intent.ID || result.Forked.ChildSession.Digest == "" {
		t.Fatalf("reconciled fork = %#v, %v", result, err)
	}
	recovered, err := ReconcileForkedSession(reopened, intent.ID, spec, config)
	if err != nil || recovered.Forked != result.Forked || !withinDirectory(spec.ChildSessionDir, recovered.ChildSessionPath) {
		t.Fatalf("recovered fork = %#v, %v", recovered, err)
	}
	if again, err := Fork(reopened, intent, spec); err != nil || again.Receipt != result.Receipt || again.Forked != result.Forked {
		t.Fatalf("idempotent fork = %#v, %v", again, err)
	}
	children, err := filepath.Glob(filepath.Join(spec.ChildSessionDir, "*.jsonl"))
	if err != nil || len(children) != 1 {
		t.Fatalf("child sessions = %#v, %v", children, err)
	}
}

func stageUnknownFork(t *testing.T, operation *run.Operation, intent effect.Intent, spec ForkSpec) error {
	t.Helper()
	checkpointRef := intentCheckpoint(t, operation, intent)
	checkpoint, prefix, session, opaque, err := operation.RuntimeCheckpointData(checkpointRef)
	if err != nil {
		return err
	}
	state, err := validateForkParent(spec, checkpoint, session, opaque, prefix, attemptIdentity(t, operation, intent))
	if err != nil {
		return err
	}
	attempt, err := newForkAttempt(spec, state, checkpoint, checkpointRef, attemptIdentity(t, operation, intent))
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	created, err := operation.BeginEffectAttempt(intent, encoded)
	if err != nil || !created {
		return err
	}
	if err := requireEmptyDirectory(spec.ChildSessionDir); err != nil {
		return err
	}
	_, err = executeForkBridge(attempt)
	return err
}

func attemptIdentity(t *testing.T, operation *run.Operation, intent effect.Intent) AdapterIdentity {
	t.Helper()
	data, err := operation.ReadEffectPayload(intent)
	if err != nil {
		t.Fatal(err)
	}
	var payload ForkPayload
	if strictjson.Decode(data, &payload) != nil {
		t.Fatal("invalid fork payload")
	}
	return payload.Identity
}

func intentCheckpoint(t *testing.T, operation *run.Operation, intent effect.Intent) artifact.Ref {
	t.Helper()
	data, err := operation.ReadEffectPayload(intent)
	if err != nil {
		t.Fatal(err)
	}
	var payload ForkPayload
	if strictjson.Decode(data, &payload) != nil {
		t.Fatal("invalid fork payload")
	}
	return payload.Checkpoint
}
