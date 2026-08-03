package experiment

import (
	"testing"

	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/source"
)

func TestDecisionBoundDomainMutationsShareOneDecisionLedger(t *testing.T) {
	root, op, _, effect := decisionFixture(t)
	findingValue := finding.Finding{ID: "finding-domain", Class: "target_mismatch", Severity: finding.SeverityHigh, Symptom: "receipt differs", Impact: "production changed", Evidence: effect.Decision.Evidence, Confidence: finding.ConfidenceHigh, Falsifier: "target agrees"}
	findingDecision := effect.Decision
	findingDecision.ID, findingDecision.Action = "finding-domain", DecisionFinding
	if err := op.RecordFindingWithDecision(DecisionBoundFinding{Decision: findingDecision, Finding: findingValue}); err != nil {
		t.Fatal(err)
	}
	current, err := op.current()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.Load(op.artifacts, current.begun.Source)
	if err != nil {
		t.Fatal(err)
	}
	diagnosed := diagnosis.Diagnosis{
		ID: "diagnosis-domain", State: diagnosis.Established, FindingIDs: []string{findingValue.ID}, SourceSnapshot: current.begun.Source,
		SourceEvidence: []diagnosis.SourceEvidenceRef{{Path: snapshot.Files[0].Path, Artifact: snapshot.Files[0].Artifact, StartLine: 1, EndLine: 3, EstablishesOwner: true}},
		Owner:          "deployment target constructor", RootCause: "target has two owners", Invariant: "one validated target flows to receipt", RepairBoundary: "target constructor", AcceptanceClaims: []diagnosis.Claim{{ID: "target-owner", Statement: "one target owner", Falsifier: "default reread"}},
	}
	diagnosisDecision := effect.Decision
	diagnosisDecision.ID, diagnosisDecision.Action = "diagnosis-domain", DecisionDiagnosis
	if err := op.RecordDiagnosisWithDecision(DecisionBoundDiagnosis{Decision: diagnosisDecision, Diagnosis: diagnosed}); err != nil {
		t.Fatal(err)
	}
	candidateRef, err := op.artifacts.Put([]byte("candidate"))
	if err != nil {
		t.Fatal(err)
	}
	candidateDecision := effect.Decision
	candidateDecision.ID, candidateDecision.Action = "candidate-domain", DecisionCandidate
	if _, err := op.BindCandidateWithDecision(DecisionBoundCandidate{Decision: candidateDecision, Candidate: diagnosis.RepairCandidate{ID: "candidate-domain", DiagnosisID: diagnosed.ID, Artifact: candidateRef}}); err != nil {
		t.Fatal(err)
	}
	continueDecision := effect.Decision
	continueDecision.ID, continueDecision.Action = "continue-domain", DecisionContinue
	if err := op.RecordContinueWithDecision(DecisionBoundContinue{Decision: continueDecision}); err != nil {
		t.Fatal(err)
	}
	if err := op.RecordContinueWithDecision(DecisionBoundContinue{Decision: continueDecision}); err == nil {
		t.Fatal("duplicate decision accepted")
	}
	reopened, err := Open(root, "decision-exp")
	if err != nil {
		t.Fatal(err)
	}
	status, err := reopened.Status()
	if err != nil || len(status.DecisionIDs) != 4 || len(status.DiagnosisIDs) != 1 || len(status.CandidateIDs) != 1 {
		t.Fatalf("status = %#v, %v", status, err)
	}
}
