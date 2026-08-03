package tool

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func renderOwnedHandoff(t *testing.T, binding Binding) artifact.Ref {
	t.Helper()
	prep, _ := preparation.Open(binding.Root, "prep")
	spec := preparation.BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "owner.go", Content: []byte("package owner")}}, Authority: "designer"}
	if _, err := prep.Begin(spec); err != nil {
		t.Fatal(err)
	}
	status, _ := prep.Status()
	store := binding.store()
	assay, _ := store.Put([]byte("assay"))
	value := preparation.LeakageAssay{WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer", Authority: "reviewer", Method: "test", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{assay}}
	if err := prep.RecordLeakageAssay(value); err != nil {
		t.Fatal(err)
	}
	basis, _ := prep.ChallengeBasis()
	if err := prep.Challenge(preparation.Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Seal(); err != nil {
		t.Fatal(err)
	}
	op, _ := experiment.Open(binding.Root, binding.ExperimentID)
	if _, err := op.Begin("prep"); err != nil {
		t.Fatal(err)
	}
	put := func(name string) artifact.Ref {
		ref, err := store.Put([]byte(name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	fixture := put("fixture")
	candidate, err := source.Build(store, []source.InputFile{{Path: "main.go", Content: []byte("package candidate\n")}})
	if err != nil {
		t.Fatal(err)
	}
	reset, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{Contract: experiment.FixtureResetContract, RunID: "worker", Fixture: fixture, Baseline: put("baseline"), Evidence: []artifact.Ref{put("reset")}})
	if err != nil {
		t.Fatal(err)
	}
	inputs := experiment.RunInputs{Harness: put("harness"), Trial: put("trial"), Candidate: candidate, Adapter: put("adapter"), OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset, EvidencePolicy: put("evidence"), StopPolicy: put("stop"), WorkerRuntime: put("runtime"), Environment: put("environment")}
	if _, err := op.BindRun("worker", experiment.NewFreshOrigin(), inputs); err != nil {
		t.Fatal(err)
	}
	worker, _ := run.Open(binding.Root, binding.ExperimentID, "worker")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := worker.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor-0"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	writer, _, err := worker.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit([]byte("cursor-1"), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: "tool_result", Raw: []byte("failed"), Label: "failure"}}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	evidence := run.EvidenceRef{ExperimentID: binding.ExperimentID, RunID: "worker", Sequence: 2, Item: 0}
	fact := finding.Finding{ID: "finding", Class: "failure", Severity: finding.SeverityHigh, Symptom: "failed", Impact: "incomplete", Evidence: []run.EvidenceRef{evidence}, Confidence: finding.ConfidenceHigh, Falsifier: "success"}
	if err := op.RecordFinding(fact); err != nil {
		t.Fatal(err)
	}
	decision := experiment.SupervisorDecision{ID: "handoff", WorkerRun: "worker", EvidenceThrough: 2, Claim: "handoff required", Action: experiment.DecisionHandoff, Evidence: []run.EvidenceRef{evidence}, Falsifier: "no handoff"}
	result, err := op.RenderHandoffWithDecision(decision, []string{fact.ID})
	if err != nil {
		t.Fatal(err)
	}
	return result.Artifact
}
