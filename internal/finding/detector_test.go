package finding

import (
	"testing"

	"github.com/yansircc/agentlab/internal/run"
)

func TestRepeatedLabelDetectorIsDeterministicAndEvidenceOnly(t *testing.T) {
	items := []run.EvidenceItem{
		{Ref: run.EvidenceRef{ExperimentID: "exp", RunID: "run", Sequence: 2, Item: 0}, Label: "tool_failure"},
		{Ref: run.EvidenceRef{ExperimentID: "exp", RunID: "run", Sequence: 3, Item: 0}, Label: "tool_failure"},
	}
	detector := RepeatedLabelDetector{Label: "tool_failure", Threshold: 2}
	first, err := detector.Detect(items)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := detector.Detect(items)
	if first == nil || second == nil || first.ID != second.ID || len(first.Evidence) != 2 {
		t.Fatalf("findings = %#v %#v", first, second)
	}
	if result, err := (RepeatedLabelDetector{Label: "tool_failure", Threshold: 3}).Detect(items); err != nil || result != nil {
		t.Fatalf("below-threshold result = %#v, %v", result, err)
	}
}

func TestAcceptedRiskRequiresHumanAuthority(t *testing.T) {
	value := Disposition{FindingID: "finding-1", Kind: AcceptedRisk, Authority: "agent", Reason: "cheap"}
	if err := value.Validate(); err == nil {
		t.Fatal("agent accepted risk for human")
	}
	value.Authority = "human"
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPublicContractDetectorFindsOnlyUnauthorizedToolCalls(t *testing.T) {
	items := []run.EvidenceItem{
		{Ref: run.EvidenceRef{ExperimentID: "exp", RunID: "run", Sequence: 2}, Kind: run.EvidenceToolCall, Label: "read"},
		{Ref: run.EvidenceRef{ExperimentID: "exp", RunID: "run", Sequence: 3}, Kind: run.EvidenceToolCall, Label: "private_shell"},
		{Ref: run.EvidenceRef{ExperimentID: "exp", RunID: "run", Sequence: 4}, Kind: run.EvidenceToolResult, Label: "private_shell"},
	}
	detector := PublicContractDetector{AllowedTools: []string{"read"}}
	result, err := detector.Detect(items)
	if err != nil || result == nil || len(result.Evidence) != 1 || result.Evidence[0] != items[1].Ref {
		t.Fatalf("public contract finding = %#v, %v", result, err)
	}
	if clean, err := (PublicContractDetector{AllowedTools: []string{"read", "private_shell"}}).Detect(items); err != nil || clean != nil {
		t.Fatalf("allowed tools produced finding = %#v, %v", clean, err)
	}
}
