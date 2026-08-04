package experiment

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func TestRequireSettledStartEffectsAcceptsDecisionBoundStart(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "start-prep")
	operation, err := Open(root, "start-experiment")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operation.Begin("start-prep"); err != nil {
		t.Fatal(err)
	}
	bindTestRun(t, operation, "worker")
	payload, err := run.EncodeStartPayload(effect.WorkerStart, run.StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "worker-start", RunID: "worker", Kind: effect.WorkerStart, Payload: payloadRef}
	decision := SupervisorDecision{ID: intent.ID, WorkerRun: "worker", Claim: "launch the sealed fresh Worker", Action: DecisionWorkerStart, Falsifier: "run has pre-start public evidence"}
	if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: decision, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	worker, err := run.Open(root, "start-experiment", "worker")
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := worker.BeginAttachedEffect(intent, run.AttachedSpec{Adapter: "test", StreamID: "worker-session", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	if err := operation.RequireSettledStartEffects(); err != nil {
		t.Fatalf("decision-bound start was rejected: %v", err)
	}
}

func TestRequireSettledStartEffectsRejectsUnboundAndRetroactiveStart(t *testing.T) {
	_, operation, worker, value := decisionFixture(t)
	if err := operation.RequireSettledStartEffects(); err == nil {
		t.Fatal("direct run start satisfied the recursive start chain")
	}
	payload, err := run.EncodeStartPayload(effect.WorkerStart, run.StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	decision := value.Decision
	decision.ID, decision.Action = "retroactive-start", DecisionWorkerStart
	intent := effect.Intent{ID: decision.ID, RunID: "worker", Kind: effect.WorkerStart, Payload: payloadRef}
	if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: decision, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.SettleEffect(intent, []byte("retroactive start receipt")); err != nil {
		t.Fatal(err)
	}
	if err := operation.RequireSettledStartEffects(); err == nil {
		t.Fatal("retroactive start decision satisfied the recursive start chain")
	}
}

func TestRequireVerifiedRuntimeEffectsAcceptsObservedStop(t *testing.T) {
	operation, worker, stop := verifiedRuntimeEffectFixture(t)
	if _, err := worker.RequestStopEffect(stop); err != nil {
		t.Fatal(err)
	}
	if err := operation.RequireVerifiedRuntimeEffects(); err != nil {
		t.Fatalf("verified runtime effects were rejected: %v", err)
	}
}

func TestRequireVerifiedRuntimeEffectsRejectsSyntheticStopReceipt(t *testing.T) {
	operation, worker, stop := verifiedRuntimeEffectFixture(t)
	if _, err := worker.SettleEffect(stop, []byte("synthetic stop receipt")); err != nil {
		t.Fatal(err)
	}
	if err := operation.RequireVerifiedRuntimeEffects(); err == nil {
		t.Fatal("synthetic stop receipt satisfied the recursive runtime gate")
	}
}

func verifiedRuntimeEffectFixture(t *testing.T) (*Operation, *run.Operation, effect.Intent) {
	t.Helper()
	root := t.TempDir()
	sealPreparation(t, root, "verified-runtime-prep")
	operation, err := Open(root, "verified-runtime-experiment")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operation.Begin("verified-runtime-prep"); err != nil {
		t.Fatal(err)
	}
	bindTestRun(t, operation, "worker")
	startPayload, err := run.EncodeStartPayload(effect.WorkerStart, run.StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	startRef, err := operation.artifacts.Put(startPayload)
	if err != nil {
		t.Fatal(err)
	}
	start := effect.Intent{ID: "worker-start", RunID: "worker", Kind: effect.WorkerStart, Payload: startRef}
	startDecision := SupervisorDecision{ID: start.ID, WorkerRun: "worker", Claim: "launch the sealed fresh Worker", Action: DecisionWorkerStart, Falsifier: "run has pre-start public evidence"}
	if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: startDecision, Intent: start}); err != nil {
		t.Fatal(err)
	}
	worker, err := run.Open(root, "verified-runtime-experiment", "worker")
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := worker.BeginAttachedEffect(start, run.AttachedSpec{Adapter: "test", StreamID: "worker-session", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	writer, _, err := worker.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit([]byte("next-cursor"), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: run.EvidenceToolResult, Label: "target_mismatch", Raw: []byte("target differs")}}}); err != nil {
		_ = writer.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	stopPayload, err := run.EncodeStopPayload(run.StopPayload{Reason: "material failure"})
	if err != nil {
		t.Fatal(err)
	}
	stopRef, err := operation.artifacts.Put(stopPayload)
	if err != nil {
		t.Fatal(err)
	}
	stop := effect.Intent{ID: "worker-stop", RunID: "worker", Kind: effect.Stop, Payload: stopRef}
	evidence := run.EvidenceRef{ExperimentID: "verified-runtime-experiment", RunID: "worker", Sequence: 2, Item: 0}
	stopDecision := SupervisorDecision{ID: stop.ID, WorkerRun: "worker", EvidenceThrough: evidence.Sequence, Claim: "the public target mismatch is material", Action: DecisionStop, Evidence: []run.EvidenceRef{evidence}, Falsifier: "target and receipt agree"}
	if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: stopDecision, Intent: stop}); err != nil {
		t.Fatal(err)
	}
	return operation, worker, stop
}
