package preparation

import (
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func TestSharedPreparationAlgebraSealsThreeMateriallyDifferentFamilies(t *testing.T) {
	families := []struct {
		id     string
		intent string
		source string
		nodes  []DecisionKind
	}{
		{id: "remote-site", intent: "inspect a remote website task", source: "workflow contract", nodes: []DecisionKind{DiscoverableFact, HumanDecision, BlockedExternalFact}},
		{id: "go-refactor", intent: "close a refactor failure class", source: "package transition", nodes: []DecisionKind{DiscoverableFact, HumanDecision, LowRiskAssumption}},
		{id: "api-debug", intent: "diagnose rejected API requests", source: "driver boundary", nodes: []DecisionKind{DiscoverableFact, BlockedExternalFact, HumanDecision}},
	}
	for _, family := range families {
		t.Run(family.id, func(t *testing.T) {
			op, _ := Open(t.TempDir(), family.id)
			_, err := op.Begin(BeginSpec{
				UserIntent: []byte(family.intent), SourceFiles: []source.InputFile{{Path: "source.txt", Content: []byte(family.source)}}, Authority: "designer",
			})
			if err != nil {
				t.Fatal(err)
			}
			evidence, _ := op.artifacts.Put([]byte("family evidence"))
			if err := op.RecordFact(RepositoryFact{ID: "repository-contract", Statement: "entrypoint discovered", Evidence: []artifact.Ref{evidence}}); err != nil {
				t.Fatal(err)
			}
			for index, kind := range family.nodes {
				node := familyNode(index, kind, evidence)
				if err := op.ProposeDecision(node); err != nil {
					t.Fatal(err)
				}
			}
			for index, kind := range family.nodes {
				resolution := familyResolution(index, kind, evidence)
				if err := op.ResolveDecision(resolution); err != nil {
					t.Fatal(err)
				}
			}
			recordCleanAssay(t, op)
			basis, _ := op.ChallengeBasis()
			if err := op.Challenge(Challenge{Basis: basis}); err != nil {
				t.Fatal(err)
			}
			if status, err := op.Seal(); err != nil || status.Phase != PhaseSealed {
				t.Fatalf("sealed family = %#v, %v", status, err)
			}
		})
	}
}

func familyNode(index int, kind DecisionKind, evidence artifact.Ref) DecisionNode {
	id := []string{"first", "second", "third"}[index]
	node := DecisionNode{ID: id}
	if index > 0 {
		node.DependsOn = []string{[]string{"first", "second"}[index-1]}
	}
	switch kind {
	case DiscoverableFact:
		node.Fact = &FactNode{Query: "inspect repository-owned fact"}
	case HumanDecision:
		node.MaterialTo = []string{"worker_input"}
		node.Human = &HumanNode{
			Question: "Which target is authoritative?", Recommended: DecisionOption{ID: "safe", Label: "Safe", Consequences: "bounded mutation", Evidence: []artifact.Ref{evidence}},
			Alternatives: []DecisionOption{{ID: "broad", Label: "Broad", Consequences: "larger mutation"}},
		}
	case LowRiskAssumption:
		node.Assumption = &AssumptionNode{Statement: "retain local evidence", Consequence: "uses bounded disk"}
	case BlockedExternalFact:
		node.MaterialTo = []string{"run_manifest"}
		node.ExternalFact = &ExternalNode{Requirement: "external identity receipt"}
	}
	return node
}

func familyResolution(index int, kind DecisionKind, evidence artifact.Ref) Resolution {
	resolution := Resolution{NodeID: []string{"first", "second", "third"}[index], Answer: "resolved"}
	switch kind {
	case DiscoverableFact:
		resolution.Authority, resolution.Evidence = "repository", []artifact.Ref{evidence}
	case HumanDecision:
		resolution.Authority, resolution.OptionID = "human", "safe"
	case LowRiskAssumption:
		resolution.Authority = "designer"
	case BlockedExternalFact:
		resolution.Authority, resolution.Evidence = "external", []artifact.Ref{evidence}
	}
	return resolution
}
