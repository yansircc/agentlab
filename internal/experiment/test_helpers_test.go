package experiment

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func sealPreparation(t *testing.T, root, id string) {
	t.Helper()
	prep, _ := preparation.Open(root, id)
	if _, err := prep.Begin(preparation.BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "owner.go", Content: []byte("package owner\n\nfunc transition() {}\n")}}, Authority: "designer"}); err != nil {
		t.Fatal(err)
	}
	status, _ := prep.Status()
	evidence, _ := artifact.NewStore(filepath.Join(root, "artifacts")).Put([]byte("independent leakage assay"))
	if err := prep.RecordLeakageAssay(preparation.LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer-1", Authority: "reviewer",
		Method: "semantic-contrast-review", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{evidence},
	}); err != nil {
		t.Fatal(err)
	}
	basis, _ := prep.ChallengeBasis()
	if err := prep.Challenge(preparation.Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Seal(); err != nil {
		t.Fatal(err)
	}
}

func attachedRunWithEvidence(t *testing.T, root, experimentID, runID string) *run.Operation {
	t.Helper()
	op, _ := run.Open(root, experimentID, runID)
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor-0"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	writer, _, err := op.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Commit([]byte("cursor-1"), run.AdapterBatch{Events: []run.AdapterEvent{
		{Kind: "tool_result", Raw: []byte("first"), Label: "validation_failure"},
		{Kind: "tool_result", Raw: []byte("second"), Label: "validation_failure"},
	}})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func bindTestRun(t *testing.T, operation *Operation, runID string) {
	t.Helper()
	put := func(name string) artifact.Ref {
		ref, err := operation.artifacts.Put([]byte(name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	fixture := put("fixture")
	reset := recordTestFixtureReset(t, operation, runID, fixture, put("fixture-baseline"))
	_, err := operation.BindRun(runID, RunInputs{
		Harness: put("harness"), Trial: put("trial"), Candidate: put("baseline"), Adapter: put("adapter"),
		OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset, EvidencePolicy: put("evidence-policy"),
		StopPolicy: put("stop-policy"), WorkerRuntime: put("runtime"), Environment: put("environment"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func recordTestFixtureReset(t *testing.T, operation *Operation, runID string, fixture, baseline artifact.Ref) artifact.Ref {
	t.Helper()
	evidence, err := operation.artifacts.Put([]byte("reset-evidence:" + runID))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := RecordFixtureReset(operation.artifacts, FixtureResetProof{
		Contract: FixtureResetContract, RunID: runID, Fixture: fixture, Baseline: baseline, Evidence: []artifact.Ref{evidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func rebindTestFixtureReset(t *testing.T, operation *Operation, runID string, inputs RunInputs) RunInputs {
	t.Helper()
	previous, err := loadFixtureReset(operation.artifacts, inputs.FixtureReset)
	if err != nil {
		t.Fatal(err)
	}
	inputs.FixtureReset = recordTestFixtureReset(t, operation, runID, inputs.Fixture, previous.Baseline)
	return inputs
}
