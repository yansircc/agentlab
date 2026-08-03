package preparation

import (
	"errors"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func TestPreparationDependencyFrontierChallengeAndSeal(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "prep-1")
	status, err := op.Begin(BeginSpec{
		UserIntent: []byte("build the requested product"), SourceFiles: []source.InputFile{{Path: "owner.go", Content: []byte("private-source-facts")}},
		PublicArtifacts: [][]byte{[]byte("public-readme")}, Authority: "designer",
	})
	if err != nil || status.Phase != PhaseExploring {
		t.Fatalf("begin = %#v, %v", status, err)
	}
	records, err := op.Inspect(0, 2)
	if err != nil || len(records) != 2 || records[0].Kind != eventWorkerInput || records[1].Kind != eventSource {
		t.Fatalf("worker/source ordering = %#v, %v", records, err)
	}
	assertWorkerInputBoundary(t, op, status)
	evidence, _ := op.artifacts.Put([]byte("repository evidence"))
	if err := op.RecordFact(RepositoryFact{ID: "repo-boundary", Statement: "public entrypoint exists", Evidence: []artifact.Ref{evidence}}); err != nil {
		t.Fatal(err)
	}
	fact := DecisionNode{ID: "discover-runtime", Fact: &FactNode{Query: "resolve runtime from repository"}}
	human := DecisionNode{
		ID: "choose-target", DependsOn: []string{fact.ID}, MaterialTo: []string{"worker_input"},
		Human: &HumanNode{
			Question:     "Which target is authoritative?",
			Recommended:  DecisionOption{ID: "staging", Label: "Staging", Consequences: "No production mutation", Reversible: true, Evidence: []artifact.Ref{evidence}},
			Alternatives: []DecisionOption{{ID: "production", Label: "Production", Consequences: "Mutates production"}},
		},
	}
	assumption := DecisionNode{ID: "retain-logs", DependsOn: []string{human.ID}, Assumption: &AssumptionNode{Statement: "retain logs", Consequence: "uses local disk"}}
	for _, node := range []DecisionNode{fact, human, assumption} {
		if err := op.ProposeDecision(node); err != nil {
			t.Fatal(err)
		}
	}
	if status, _ := op.Status(); status.NextNode == nil || status.NextNode.ID != fact.ID || status.Phase != PhaseExploring {
		t.Fatalf("fact frontier = %#v", status)
	}
	if err := op.ResolveDecision(Resolution{NodeID: human.ID, Answer: "staging", OptionID: "staging", Authority: "human"}); !errors.Is(err, ErrWrongFrontier) {
		t.Fatalf("out-of-order resolution = %v", err)
	}
	if err := op.ResolveDecision(Resolution{NodeID: fact.ID, Answer: "Pi", Authority: "repository", Evidence: []artifact.Ref{evidence}}); err != nil {
		t.Fatal(err)
	}
	if status, _ := op.Status(); status.NextNode == nil || status.NextNode.ID != human.ID || status.Phase != PhaseNeedsDecision {
		t.Fatalf("human frontier = %#v", status)
	}
	if err := op.ResolveDecision(Resolution{NodeID: human.ID, Answer: "staging", OptionID: "staging", Authority: "designer"}); err == nil {
		t.Fatal("designer resolved human decision")
	}
	if err := op.ResolveDecision(Resolution{NodeID: human.ID, Answer: "staging", OptionID: "staging", Authority: "human"}); err != nil {
		t.Fatal(err)
	}
	if err := op.ResolveDecision(Resolution{NodeID: assumption.ID, Answer: "accepted", Authority: "designer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Seal(); !errors.Is(err, ErrLeakageAssayRequired) {
		t.Fatalf("seal without leakage assay = %v", err)
	}
	recordCleanAssay(t, op)
	if _, err := op.Seal(); !errors.Is(err, ErrChallengeNeeded) {
		t.Fatalf("seal without challenge = %v", err)
	}
	basis, _ := op.ChallengeBasis()
	if err := op.Challenge(Challenge{Basis: basis, Gaps: []ChallengeGap{{ID: "missing-reset", Statement: "reset proof is absent"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Seal(); !errors.Is(err, ErrChallengeOpen) {
		t.Fatalf("seal with gap = %v", err)
	}
	basis, _ = op.ChallengeBasis()
	if err := op.Challenge(Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	sealedStatus, err := op.Seal()
	if err != nil || sealedStatus.Phase != PhaseSealed {
		t.Fatalf("seal = %#v, %v", sealedStatus, err)
	}
	reopened, _ := Open(root, "prep-1")
	if status, err := reopened.Status(); err != nil || status.Phase != PhaseSealed || status.WorkerInput != sealedStatus.WorkerInput {
		t.Fatalf("replayed status = %#v, %v", status, err)
	}
}

func TestChallengeIsBoundToExactPreparationPrefix(t *testing.T) {
	op := begunOperation(t, "basis")
	recordCleanAssay(t, op)
	basis, _ := op.ChallengeBasis()
	if err := op.Challenge(Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	evidence, _ := op.artifacts.Put([]byte("new evidence"))
	if err := op.RecordFact(RepositoryFact{ID: "late-fact", Statement: "added after review", Evidence: []artifact.Ref{evidence}}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Seal(); !errors.Is(err, ErrChallengeNeeded) {
		t.Fatalf("stale clean challenge admitted seal: %v", err)
	}
	if err := op.Challenge(Challenge{Basis: basis}); err == nil {
		t.Fatal("stale challenge basis was accepted")
	}
}

func TestBeginRecoversOnlyExactWorkerInputSeal(t *testing.T) {
	op, spec := partialBegin(t, "resume-begin")
	if status, err := op.Begin(spec); err != nil || status.Source.Digest == "" || status.EventCount != 2 {
		t.Fatalf("exact begin recovery = %#v, %v", status, err)
	}

	op, spec = partialBegin(t, "conflicting-begin")
	spec.UserIntent = []byte("different intent")
	if _, err := op.Begin(spec); !errors.Is(err, ErrAlreadyBegun) {
		t.Fatalf("conflicting partial begin = %v", err)
	}
	if records, err := op.Inspect(0, 10); err != nil || len(records) != 1 || records[0].Kind != eventWorkerInput {
		t.Fatalf("conflicting begin appended state: %#v, %v", records, err)
	}
}

func TestDecisionVariantsDependenciesAndSingleFrontierFailClosed(t *testing.T) {
	op := begunOperation(t, "invalid")
	invalid := DecisionNode{ID: "two-kinds", Fact: &FactNode{Query: "q"}, Assumption: &AssumptionNode{Statement: "s", Consequence: "c"}}
	if err := op.ProposeDecision(invalid); err == nil {
		t.Fatal("multi-variant node was accepted")
	}
	if err := op.ProposeDecision(DecisionNode{ID: "forward", DependsOn: []string{"missing"}, Fact: &FactNode{Query: "q"}}); err == nil {
		t.Fatal("forward dependency was accepted")
	}
	evidence, _ := op.artifacts.Put([]byte("recommendation evidence"))
	first := humanDecision("first", evidence)
	second := humanDecision("second", evidence)
	if err := op.ProposeDecision(first); err != nil {
		t.Fatal(err)
	}
	if err := op.ProposeDecision(second); err != nil {
		t.Fatal(err)
	}
	if err := op.ResolveDecision(Resolution{NodeID: second.ID, Answer: "yes", OptionID: "yes", Authority: "human"}); !errors.Is(err, ErrWrongFrontier) {
		t.Fatalf("second human became simultaneously actionable: %v", err)
	}
}

func humanDecision(id string, evidence artifact.Ref) DecisionNode {
	return DecisionNode{ID: id, MaterialTo: []string{"worker_input"}, Human: &HumanNode{
		Question: "Proceed?", Recommended: DecisionOption{ID: "yes", Label: "Yes", Consequences: "proceed", Evidence: []artifact.Ref{evidence}},
		Alternatives: []DecisionOption{{ID: "no", Label: "No", Consequences: "stop"}},
	}}
}
