package run

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
)

func TestAttachedWorkerStartEffectSettlesOneIntent(t *testing.T) {
	op, _ := Open(t.TempDir(), "effect-experiment", "effect-run")
	bindTestManifest(t, op)
	payload, err := EncodeStartPayload(effect.WorkerStart, StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "worker-start", RunID: "effect-run", Kind: effect.WorkerStart, Payload: ref}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	spec := AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}
	first, err := op.BeginAttachedEffect(intent, spec)
	if err != nil || first.Receipt.IntentID != intent.ID || first.State.StreamID != spec.StreamID {
		t.Fatalf("first start = %#v, %v", first, err)
	}
	second, err := op.BeginAttachedEffect(intent, spec)
	if err != nil || second.Receipt != first.Receipt || second.State.Adapter != first.State.Adapter || second.State.StreamID != first.State.StreamID {
		t.Fatalf("reconciled start = %#v, %v", second, err)
	}
}

func TestCoderStartRequiresBoundedHandoff(t *testing.T) {
	if _, err := EncodeStartPayload(effect.CoderStart, StartPayload{}); err == nil {
		t.Fatal("coder start accepted no handoff")
	}
}
