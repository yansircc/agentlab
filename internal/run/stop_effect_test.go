package run

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
)

func TestStopEffectReconcilesOneDurableStopRequest(t *testing.T) {
	op, _ := Open(t.TempDir(), "effect-experiment", "effect-run")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeStopPayload(StopPayload{Reason: "material_failure"})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "stop-effect", RunID: "effect-run", Kind: effect.Stop, Payload: ref}
	first, err := op.RequestStopEffect(intent)
	if err != nil || !first.Stop.Admitted || first.Receipt.IntentID != intent.ID {
		t.Fatalf("first stop = %#v, %v", first, err)
	}
	second, err := op.RequestStopEffect(intent)
	if err != nil || second != first {
		t.Fatalf("reconciled stop = %#v, %v", second, err)
	}
	verified, err := op.VerifyStopEffect(intent)
	if err != nil || verified != first.Stop {
		t.Fatalf("verified stop = %#v, %v", verified, err)
	}
}

func TestVerifyStopEffectRejectsSyntheticReceipt(t *testing.T) {
	op, _ := Open(t.TempDir(), "effect-experiment", "synthetic-stop")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeStopPayload(StopPayload{Reason: "material_failure"})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "synthetic-stop", RunID: "synthetic-stop", Kind: effect.Stop, Payload: ref}
	if _, err := op.SettleEffect(intent, []byte("synthetic receipt")); err != nil {
		t.Fatal(err)
	}
	if _, err := op.VerifyStopEffect(intent); err == nil {
		t.Fatal("synthetic stop receipt was accepted")
	}
}
