package run

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
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
	if err := op.VerifyStartEffect(intent); err != nil {
		t.Fatalf("start effect verification = %v", err)
	}
}

func TestVerifyStartEffectRejectsReceiptWithoutStartObservation(t *testing.T) {
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
	intent := effect.Intent{ID: "retroactive-worker-start", RunID: "effect-run", Kind: effect.WorkerStart, Payload: ref}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.SettleEffect(intent, []byte("fabricated start receipt")); err != nil {
		t.Fatal(err)
	}
	if err := op.VerifyStartEffect(intent); err == nil {
		t.Fatal("start receipt without a matching start observation was accepted")
	}
}

func TestCoderStartRequiresBoundedHandoff(t *testing.T) {
	if _, err := EncodeStartPayload(effect.CoderStart, StartPayload{}); err == nil {
		t.Fatal("coder start accepted no handoff")
	}
	if _, err := EncodeStartPayload(effect.WorkerStart, StartPayload{Coder: &CoderProfile{}}); err == nil {
		t.Fatal("worker start accepted coder profile")
	}
}

func TestCoderStartReceiptBindsHostProfile(t *testing.T) {
	op, _ := Open(t.TempDir(), "effect-experiment", "effect-run")
	bindTestManifest(t, op)
	put := func(name string) artifact.Ref {
		ref, err := op.artifacts.Put([]byte(name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	profile := CoderProfile{Handoff: put("handoff"), SourceSnapshot: put("source"), CandidateWorkspace: put("workspace"), CapabilityProfile: put("profile")}
	payload, err := EncodeStartPayload(effect.CoderStart, StartPayload{Coder: &profile})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "coder-start", RunID: "effect-run", Kind: effect.CoderStart, Payload: ref}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	result, err := op.BeginAttachedEffect(intent, AttachedSpec{Adapter: "test", StreamID: "coder-session", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := op.artifacts.Read(result.Receipt.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	var observed startObservation
	if strictjson.Decode(evidence, &observed) != nil || observed.Coder == nil || *observed.Coder != profile || observed.State.StreamID != "coder-session" {
		t.Fatalf("receipt = %#v", observed)
	}
}
