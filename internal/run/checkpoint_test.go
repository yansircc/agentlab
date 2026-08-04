package run

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
)

func TestRuntimeCheckpointIsOwnedByOneActiveAdapterRun(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "checkpoint")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	value := RuntimeCheckpointSpec{Adapter: "test", Session: []byte("session"), OpaqueState: []byte("opaque"), PublicPrefix: []byte("public-prefix")}
	ref, err := op.RecordRuntimeCheckpoint(checkpointIntent(t, op, "checkpoint"), value)
	if err != nil {
		t.Fatal(err)
	}
	if owned, err := op.HasRuntimeCheckpoint(ref.Checkpoint); err != nil || !owned {
		t.Fatalf("checkpoint ownership = %t, %v", owned, err)
	}
	value.Adapter = "other"
	if _, err := op.RecordRuntimeCheckpoint(checkpointIntent(t, op, "foreign"), value); err == nil {
		t.Fatal("foreign adapter checkpoint was accepted")
	}
}

func TestRuntimeCheckpointRemainsAdmissibleAfterDurableStop(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "checkpoint-after-stop")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeStopPayload(StopPayload{Reason: "preserve the public prefix for repair"})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.RequestStopEffect(effect.Intent{ID: "stop", RunID: "checkpoint-after-stop", Kind: effect.Stop, Payload: ref}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := op.RecordRuntimeCheckpoint(checkpointIntent(t, op, "checkpoint"), RuntimeCheckpointSpec{Adapter: "test", Session: []byte("session"), OpaqueState: []byte("opaque"), PublicPrefix: []byte("public-prefix")})
	if err != nil {
		t.Fatalf("checkpoint after durable stop: %v", err)
	}
	if owned, err := op.HasRuntimeCheckpoint(checkpoint.Checkpoint); err != nil || !owned {
		t.Fatalf("replayed checkpoint after durable stop = %t, %v", owned, err)
	}
}

func TestRuntimeCheckpointReceiptRequiresMatchingPriorObservation(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "checkpoint-observation")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	intent := checkpointIntent(t, op, "checkpoint")
	if _, err := op.RecordRuntimeCheckpoint(intent, RuntimeCheckpointSpec{Adapter: "test", Session: []byte("session"), OpaqueState: []byte("opaque"), PublicPrefix: []byte("public-prefix")}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.SettleEffect(intent, []byte("checkpoint receipt")); err == nil {
		t.Fatal("checkpoint receipt was accepted without an observation")
	}
	if err := op.RecordEffectObservation(intent, []byte("different observation")); err != nil {
		t.Fatal(err)
	}
	if _, err := op.SettleEffect(intent, []byte("checkpoint receipt")); err == nil {
		t.Fatal("checkpoint receipt was accepted with a different observation")
	}
}

func checkpointIntent(t *testing.T, op *Operation, id string) effect.Intent {
	t.Helper()
	payload, err := op.artifacts.Put([]byte("checkpoint payload " + id))
	if err != nil {
		t.Fatal(err)
	}
	return effect.Intent{ID: id, RunID: op.runID, Kind: effect.Checkpoint, Payload: payload}
}
