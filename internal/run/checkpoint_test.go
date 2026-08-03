package run

import (
	"testing"
	"time"
)

func TestRuntimeCheckpointIsOwnedByOneActiveAdapterRun(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "checkpoint")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	value := RuntimeCheckpointSpec{Adapter: "test", Session: []byte("session"), OpaqueState: []byte("opaque"), PublicPrefix: []byte("public-prefix")}
	ref, err := op.RecordRuntimeCheckpoint(value)
	if err != nil {
		t.Fatal(err)
	}
	if owned, err := op.HasRuntimeCheckpoint(ref.Checkpoint); err != nil || !owned {
		t.Fatalf("checkpoint ownership = %t, %v", owned, err)
	}
	value.Adapter = "other"
	if _, err := op.RecordRuntimeCheckpoint(value); err == nil {
		t.Fatal("foreign adapter checkpoint was accepted")
	}
}
