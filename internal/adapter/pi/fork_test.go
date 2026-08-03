package pi

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
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
	identity, err := DiscoverIdentity(IdentityConfig{SDKRoot: sdkRoot, AdapterDigest: strings.Repeat("a", 64), Provider: "test", Model: "test", ThinkingPolicy: "off", CompactionPolicy: "off"})
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
	spec := ForkSpec{SDKRoot: sdkRoot, ParentSession: parentPath, ChildSessionDir: filepath.Join(dir, "children")}
	if err := stageUnknownFork(t, operation, intent, spec); err != nil {
		t.Fatal(err)
	}
	reopened, _ := run.Open(root, "fork-exp", "parent-run")
	result, err := Fork(reopened, intent, spec)
	if err != nil || result.Receipt.IntentID != intent.ID || result.Forked.ChildSession.Digest == "" {
		t.Fatalf("reconciled fork = %#v, %v", result, err)
	}
	if again, err := Fork(reopened, intent, spec); err != nil || again.Receipt != result.Receipt {
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
	if decodeForkJSON(data, &payload) != nil {
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
	if decodeForkJSON(data, &payload) != nil {
		t.Fatal("invalid fork payload")
	}
	return payload.Checkpoint
}
