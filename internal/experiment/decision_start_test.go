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
