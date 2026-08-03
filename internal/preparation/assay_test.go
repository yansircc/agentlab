package preparation

import (
	"errors"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestDetectedLeakageMakesPreparationUnsealable(t *testing.T) {
	op := begunOperation(t, "leak-detected")
	status, _ := op.Status()
	evidence, _ := op.artifacts.Put([]byte("worker input reveals source-only owner name"))
	assay := LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source,
		Reviewer: "reviewer-1", Authority: "reviewer", Method: "semantic-contrast-review",
		Verdict: LeakageDetected, Evidence: []artifact.Ref{evidence},
	}
	if err := op.RecordLeakageAssay(assay); err != nil {
		t.Fatal(err)
	}
	basis, _ := op.ChallengeBasis()
	if err := op.Challenge(Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Seal(); !errors.Is(err, ErrLeakageDetected) {
		t.Fatalf("detected leakage seal error = %v", err)
	}
	if current, _ := op.Status(); current.Phase != PhaseBlocked || current.LeakageAssay == nil {
		t.Fatalf("detected leakage status = %#v", current)
	}
}

func TestLeakageAssayRequiresIndependentExactEvidence(t *testing.T) {
	op := begunOperation(t, "leak-invalid")
	status, _ := op.Status()
	evidence, _ := op.artifacts.Put([]byte("assay evidence"))
	base := LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source,
		Reviewer: "designer", Authority: "reviewer", Method: "semantic-contrast-review",
		Verdict: LeakageClean, Evidence: []artifact.Ref{evidence},
	}
	if err := op.RecordLeakageAssay(base); err == nil {
		t.Fatal("preparation authority reviewed its own worker input")
	}
	base.Reviewer = "reviewer-1"
	base.WorkerInput = evidence
	if err := op.RecordLeakageAssay(base); err == nil {
		t.Fatal("assay for different worker input was accepted")
	}
	base.WorkerInput = status.WorkerInput
	base.Evidence = []artifact.Ref{{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}}
	if err := op.RecordLeakageAssay(base); err == nil {
		t.Fatal("assay with absent evidence was accepted")
	}
}
