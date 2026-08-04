package experiment

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/run"
)

func TestReplayRejectsCoderStartWithoutExperimentOwnedHandoff(t *testing.T) {
	_, operation, _, effectValue := decisionFixture(t)
	bindTestRun(t, operation, "coder")
	current, err := operation.current()
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := operation.artifacts.Put([]byte("unowned handoff"))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := operation.artifacts.Put([]byte("candidate workspace receipt"))
	if err != nil {
		t.Fatal(err)
	}
	capability, err := operation.artifacts.Put([]byte("candidate capability profile"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := run.EncodeStartPayload(effect.CoderStart, run.StartPayload{Coder: &run.CoderProfile{
		Handoff: handoff, SourceSnapshot: current.begun.Source, CandidateWorkspace: workspace, CapabilityProfile: capability,
	}})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	decision := effectValue.Decision
	decision.ID, decision.Action = "unowned-coder-start", DecisionCoderStart
	intent := effect.Intent{ID: decision.ID, RunID: "coder", Kind: effect.CoderStart, Payload: payloadRef}
	if _, err := operation.ledger.Append(time.Now().UTC(), eventDecisionEffect, DecisionBoundEffect{Decision: decision, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	if _, err := operation.Status(); err == nil {
		t.Fatal("replay accepted a Coder start without an experiment-owned handoff")
	}
}

func TestReplayRejectsCoderStartFromForeignHandoffOwner(t *testing.T) {
	root, operation, _, effectValue := decisionFixture(t)
	findingValue := finding.Finding{ID: "foreign-handoff-finding", Class: "target_mismatch", Severity: finding.SeverityHigh, Symptom: "receipt differs", Impact: "repair is required", Evidence: effectValue.Decision.Evidence, Confidence: finding.ConfidenceHigh, Falsifier: "target agrees"}
	findingDecision := effectValue.Decision
	findingDecision.ID, findingDecision.Action = findingValue.ID, DecisionFinding
	if err := operation.RecordFindingWithDecision(DecisionBoundFinding{Decision: findingDecision, Finding: findingValue}); err != nil {
		t.Fatal(err)
	}
	handoffDecision := effectValue.Decision
	handoffDecision.ID, handoffDecision.Action = "foreign-handoff", DecisionHandoff
	handoff, err := operation.RenderHandoffWithDecision(handoffDecision, []string{findingValue.ID})
	if err != nil {
		t.Fatal(err)
	}
	bindTestRun(t, operation, "other-worker")
	attachedRunWithEvidence(t, root, "decision-exp", "other-worker")
	bindTestRun(t, operation, "coder")
	current, err := operation.current()
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := operation.artifacts.Put([]byte("candidate workspace receipt"))
	if err != nil {
		t.Fatal(err)
	}
	capability, err := operation.artifacts.Put([]byte("candidate capability profile"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := run.EncodeStartPayload(effect.CoderStart, run.StartPayload{Coder: &run.CoderProfile{
		Handoff: handoff.Artifact, SourceSnapshot: current.begun.Source, CandidateWorkspace: workspace, CapabilityProfile: capability,
	}})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	decision := effectValue.Decision
	decision.ID, decision.WorkerRun, decision.Action = "foreign-coder-start", "other-worker", DecisionCoderStart
	decision.Evidence = []run.EvidenceRef{{ExperimentID: "decision-exp", RunID: "other-worker", Sequence: 2, Item: 0}}
	intent := effect.Intent{ID: decision.ID, RunID: "coder", Kind: effect.CoderStart, Payload: payloadRef}
	if _, err := operation.ledger.Append(time.Now().UTC(), eventDecisionEffect, DecisionBoundEffect{Decision: decision, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	if _, err := operation.Status(); err == nil {
		t.Fatal("replay accepted a Coder start whose decision cites another Worker run")
	}
}

func TestCoderCompletionRejectsUndecidedStart(t *testing.T) {
	_, operation, _, effectValue := decisionFixture(t)
	findingValue := finding.Finding{ID: "undecided-coder-finding", Class: "target_mismatch", Severity: finding.SeverityHigh, Symptom: "receipt differs", Impact: "repair is required", Evidence: effectValue.Decision.Evidence, Confidence: finding.ConfidenceHigh, Falsifier: "target agrees"}
	findingDecision := effectValue.Decision
	findingDecision.ID, findingDecision.Action = findingValue.ID, DecisionFinding
	if err := operation.RecordFindingWithDecision(DecisionBoundFinding{Decision: findingDecision, Finding: findingValue}); err != nil {
		t.Fatal(err)
	}
	handoffDecision := effectValue.Decision
	handoffDecision.ID, handoffDecision.Action = "undecided-coder-handoff", DecisionHandoff
	handoff, err := operation.RenderHandoffWithDecision(handoffDecision, []string{findingValue.ID})
	if err != nil {
		t.Fatal(err)
	}
	candidate := testCandidate(t, operation, "undecided-coder")
	completion := completeCandidateWithoutDecision(t, operation, "undecided-coder", handoff.Artifact, candidate)
	coder, err := run.Open(operation.root, operation.id, "undecided-coder")
	if err != nil {
		t.Fatal(err)
	}
	_, terminal, err := coder.CoderCompletionReceipt()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operation.CoderStartForCompletion("undecided-coder", terminal); err == nil {
		t.Fatal("terminal completion accepted a Coder start without a decision-bound effect")
	}
	if _, err := operation.coderCompletion("undecided-coder", completion); err == nil {
		t.Fatal("candidate completion accepted a Coder start without a decision-bound effect")
	}
}
