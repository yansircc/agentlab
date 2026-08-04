package run

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
)

func TestForkReceiptRequiresExactIntentAndPriorObservation(t *testing.T) {
	op, forkIntent, forked := forkReceiptFixture(t)
	evidence, err := json.Marshal(forked)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.SettleEffect(forkIntent, evidence); err == nil {
		t.Fatal("fork receipt was accepted without a prior observation")
	}
	if err := op.RecordEffectObservation(forkIntent, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := op.SettleEffect(forkIntent, evidence); err != nil {
		t.Fatal(err)
	}
	stored, child, receipt, err := op.ForkReceipt(forkIntent.ID)
	if err != nil || stored != forked || string(child) != "child-session" || receipt.IntentID != forkIntent.ID {
		t.Fatalf("fork receipt = %#v, %q, %#v, %v", stored, child, receipt, err)
	}
	if at, err := op.SessionForkedTime(forked.ChildSession); err != nil || at.IsZero() {
		t.Fatalf("fork receipt time = %v, %v", at, err)
	}
}

func TestForkReceiptRejectsDifferentReceiptIntent(t *testing.T) {
	op, forkIntent, forked := forkReceiptFixture(t)
	forged := forked
	forged.Intent.ID = "other-fork"
	evidence, err := json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.RecordEffectObservation(forkIntent, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := op.SettleEffect(forkIntent, evidence); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := op.ForkReceipt(forkIntent.ID); err == nil {
		t.Fatal("fork receipt accepted evidence for a different intent")
	}
}

func forkReceiptFixture(t *testing.T) (*Operation, effect.Intent, SessionForked) {
	t.Helper()
	op, _ := Open(t.TempDir(), "experiment", "fork")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := op.RecordRuntimeCheckpoint(checkpointIntent(t, op, "checkpoint"), RuntimeCheckpointSpec{Adapter: "test", Session: []byte("parent-session"), OpaqueState: []byte("parent-opaque"), PublicPrefix: []byte("public-prefix")})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := op.artifacts.Put([]byte("fork payload"))
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "fork", RunID: "fork", Kind: effect.Fork, Payload: payload}
	forked, err := op.RecordSessionForked(intent, SessionForkSpec{ExpectedCheckpoint: checkpoint.Checkpoint, ChildSession: []byte("child-session"), ObservedPrefix: []byte("public-prefix"), AdapterIdentity: []byte("adapter-identity")})
	if err != nil {
		t.Fatal(err)
	}
	return op, intent, forked
}
