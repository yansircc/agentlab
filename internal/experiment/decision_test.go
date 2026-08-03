package experiment

import (
	"testing"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/run"
)

func TestDecisionBoundEffectIsAtomicAndSettlesOnlyItsIntent(t *testing.T) {
	root, operation, runOperation, value := decisionFixture(t)
	if err := operation.CommitDecisionBoundEffect(value); err != nil {
		t.Fatal(err)
	}
	status, err := operation.Status()
	if err != nil || len(status.DecisionIDs) != 1 || status.DecisionIDs[0] != value.Intent.ID {
		t.Fatalf("status = %#v, %v", status, err)
	}
	if settlement, err := operation.EffectSettlement(); err != nil || len(settlement.Pending) != 1 {
		t.Fatalf("pending settlement = %#v, %v", settlement, err)
	}
	if _, err := runOperation.SettleEffect(value.Intent, []byte("stopped")); err != nil {
		t.Fatal(err)
	}
	reopened, _ := Open(root, "decision-exp")
	if settlement, err := reopened.EffectSettlement(); err != nil || len(settlement.Pending) != 0 || len(settlement.Orphan) != 0 || len(settlement.Mismatched) != 0 {
		t.Fatalf("settlement = %#v, %v", settlement, err)
	}
}

func TestDecisionBoundEffectRejectsHindsightAndMismatchedReceipt(t *testing.T) {
	_, operation, runOperation, value := decisionFixture(t)
	value.Decision.EvidenceThrough--
	if err := operation.CommitDecisionBoundEffect(value); err == nil {
		t.Fatal("decision accepted evidence after its prefix")
	}
	_, operation, runOperation, value = decisionFixture(t)
	if err := operation.CommitDecisionBoundEffect(value); err != nil {
		t.Fatal(err)
	}
	mismatch := value.Intent
	mismatch.Kind = effect.Checkpoint
	if _, err := runOperation.SettleEffect(mismatch, []byte("wrong effect")); err != nil {
		t.Fatal(err)
	}
	settlement, err := operation.EffectSettlement()
	if err != nil || len(settlement.Pending) != 1 || len(settlement.Mismatched) != 1 {
		t.Fatalf("mismatched settlement = %#v, %v", settlement, err)
	}
	spec := gate.Spec{ID: "gate", CandidateID: "missing", Items: []gate.Item{{
		ID: "effect", Status: gate.Blocked, Statement: "effect mismatch", Impact: "gate must close", Evidence: value.Decision.Evidence,
		Severity: finding.SeverityHigh, Confidence: finding.ConfidenceHigh, Falsifier: "matching receipt",
	}}}
	if _, err := operation.RecordGate(spec); err == nil {
		t.Fatal("gate accepted mismatched effect receipt")
	}
}

func TestEffectSettlementRejectsOrphanReceipt(t *testing.T) {
	_, operation, runOperation, value := decisionFixture(t)
	orphan := value.Intent
	orphan.ID = "orphan-effect"
	if _, err := runOperation.SettleEffect(orphan, []byte("unadmitted")); err != nil {
		t.Fatal(err)
	}
	settlement, err := operation.EffectSettlement()
	if err != nil || len(settlement.Orphan) != 1 || len(settlement.Pending) != 0 {
		t.Fatalf("orphan settlement = %#v, %v", settlement, err)
	}
}

func TestFindingCannotBeSeparatedFromItsSupervisorDecision(t *testing.T) {
	_, operation, _, value := decisionFixture(t)
	value.Decision.Action = DecisionFinding
	bound := DecisionBoundFinding{Decision: value.Decision, Finding: finding.Finding{
		ID: "finding-1", Class: "target_mismatch", Severity: finding.SeverityHigh, Symptom: "receipt differs from target", Impact: "production changed",
		Evidence: value.Decision.Evidence, Confidence: finding.ConfidenceHigh, Falsifier: "target and receipt agree",
	}}
	if err := operation.RecordFindingWithDecision(bound); err != nil {
		t.Fatal(err)
	}
	if err := operation.RecordFindingWithDecision(bound); err == nil {
		t.Fatal("duplicate decision-bound finding was accepted")
	}
	status, err := operation.Status()
	if err != nil || len(status.DecisionIDs) != 1 || len(status.FindingIDs) != 1 {
		t.Fatalf("status = %#v, %v", status, err)
	}
}

func decisionFixture(t *testing.T) (string, *Operation, *run.Operation, DecisionBoundEffect) {
	t.Helper()
	root := t.TempDir()
	sealPreparation(t, root, "decision-prep")
	operation, _ := Open(root, "decision-exp")
	if _, err := operation.Begin("decision-prep"); err != nil {
		t.Fatal(err)
	}
	bindTestRun(t, operation, "worker")
	runOperation := attachedRunWithEvidence(t, root, "decision-exp", "worker")
	payload := putTestArtifact(t, operation, "stop request")
	value := DecisionBoundEffect{
		Decision: SupervisorDecision{ID: "stop-1", WorkerRun: "worker", EvidenceThrough: 2, Claim: "target mismatch is material", Action: DecisionStop,
			Evidence: []run.EvidenceRef{{ExperimentID: "decision-exp", RunID: "worker", Sequence: 2, Item: 0}}, Falsifier: "target and receipt agree"},
		Intent: effect.Intent{ID: "stop-1", RunID: "worker", Kind: effect.Stop, Payload: payload},
	}
	return root, operation, runOperation, value
}
