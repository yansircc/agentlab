package gate

import (
	"testing"

	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/run"
)

func TestGateVerdictAndBlockerFindingShareOneSpec(t *testing.T) {
	item := Item{
		ID: "verification", Status: Blocked, Statement: "verification unavailable", Impact: "candidate cannot ship",
		Evidence: []run.EvidenceRef{{ExperimentID: "exp", RunID: "run", Sequence: 1}},
		Severity: finding.SeverityHigh, Confidence: finding.ConfidenceHigh, Falsifier: "verification passes",
	}
	spec := Spec{ID: "gate-1", CandidateID: "candidate-1", Items: []Item{item}}
	if err := spec.Validate(); err != nil || spec.Verdict() != Block {
		t.Fatalf("blocked spec = %#v, %v", spec, err)
	}
	findings := spec.BlockerFindings()
	if len(findings) != 1 || findings[0].ID != "gate-1.verification" || findings[0].Evidence[0] != item.Evidence[0] {
		t.Fatalf("blocker findings = %#v", findings)
	}
}

func TestPassingGateRequiresComparison(t *testing.T) {
	item := Item{
		ID: "claim", Status: Passed, Statement: "claim passed", Impact: "candidate may ship",
		Evidence: []run.EvidenceRef{{ExperimentID: "exp", RunID: "run", Sequence: 1}},
		Severity: finding.SeverityHigh, Confidence: finding.ConfidenceHigh, Falsifier: "claim fails",
	}
	if err := (Spec{ID: "gate-1", CandidateID: "candidate-1", Items: []Item{item}}).Validate(); err == nil {
		t.Fatal("passing gate without comparison accepted")
	}
}
