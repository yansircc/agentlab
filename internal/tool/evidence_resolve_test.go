package tool

import (
	"testing"

	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
)

// TestResolveDecisionEvidencePinsTheCitedPrefix proves the Host resolution of
// a model decision that names evidence_through without an explicit evidence
// array: it cites exactly that run event at item zero, never fabricated.
func TestResolveDecisionEvidencePinsTheCitedPrefix(t *testing.T) {
	binding := Binding{ExperimentID: "exp"}
	decision := experiment.SupervisorDecision{ID: "stop", WorkerRun: "baseline-worker", EvidenceThrough: 8, Claim: "c", Action: "stop", Falsifier: "f"}
	resolved := resolveDecisionEvidence(binding, decision)
	want := []run.EvidenceRef{{ExperimentID: "exp", RunID: "baseline-worker", Sequence: 8, Item: 0}}
	if len(resolved.Evidence) != 1 || resolved.Evidence[0] != want[0] {
		t.Fatalf("resolved evidence = %#v", resolved.Evidence)
	}
	if resolved.Validate() != nil {
		t.Fatal("resolved evidence shape must be valid; run-event existence is verified at commit")
	}
	decision.Evidence = want
	if got := resolveDecisionEvidence(binding, decision); len(got.Evidence) != 1 || got.Evidence[0] != want[0] {
		t.Fatalf("explicit evidence was overwritten: %#v", got.Evidence)
	}
	bootstrap := experiment.SupervisorDecision{ID: "s", WorkerRun: "w", Claim: "c", Action: "worker_start", Falsifier: "f"}
	if got := resolveDecisionEvidence(binding, bootstrap); len(got.Evidence) != 0 {
		t.Fatalf("bootstrap decision gained evidence: %#v", got.Evidence)
	}
}
