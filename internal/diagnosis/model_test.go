package diagnosis

import (
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestEstablishedDiagnosisRequiresSingleSourceEvidenceOwner(t *testing.T) {
	ref := artifact.Ref{Scope: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 10}
	value := Diagnosis{
		ID: "diagnosis-1", State: Established, FindingIDs: []string{"finding-1"}, SourceSnapshot: ref,
		SourceEvidence: []SourceEvidenceRef{{Artifact: ref, Path: "owner.go", StartLine: 1, EndLine: 1}},
		Owner:          "owner", RootCause: "cause", Invariant: "invariant", RepairBoundary: "boundary",
		AcceptanceClaims: []Claim{{ID: "claim-1", Statement: "class closed", Falsifier: "counterexample"}},
	}
	if err := value.Validate(); err == nil {
		t.Fatal("established diagnosis without owner evidence was accepted")
	}
	value.SourceEvidence[0].EstablishesOwner = true
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	value.SourceEvidence[0].Path = "../outside.go"
	if err := value.Validate(); err == nil {
		t.Fatal("outside source evidence path was accepted")
	}
}
