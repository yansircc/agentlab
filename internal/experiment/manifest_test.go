package experiment

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
)

func TestRunManifestCannotChangeForOneRun(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "manifest-prep")
	operation, _ := Open(root, "manifest-exp")
	_, _ = operation.Begin("manifest-prep")
	inputs := testRunInputs(t, operation, "run-1", "first")
	first := bindPreparedTestRun(t, operation, "run-1", NewFreshOrigin(), inputs)
	if same := bindPreparedTestRun(t, operation, "run-1", NewFreshOrigin(), inputs); same != first {
		t.Fatalf("exact binding was not idempotent: %#v", same)
	}
	changed := inputs
	changed.Trial = putTestArtifact(t, operation, "changed-trial")
	prepared, err := RecordPreparedRun(operation.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: "run-1", Inputs: changed})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operation.BindPreparedRun("run-1", NewFreshOrigin(), prepared); err == nil {
		t.Fatal("changed manifest was accepted for bound run")
	}
}

func TestRunManifestCannotBindAfterRunEvent(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "event-prep")
	operation, _ := Open(root, "event-exp")
	_, _ = operation.Begin("event-prep")
	inputs := testRunInputs(t, operation, "run-1", "bound")
	bindPreparedTestRun(t, operation, "run-1", NewFreshOrigin(), inputs)
	runOperation, _ := run.Open(root, "event-exp", "run-1")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := runOperation.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	prepared, err := RecordPreparedRun(operation.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: "run-1", Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operation.BindPreparedRun("run-1", NewFreshOrigin(), prepared); err == nil {
		t.Fatal("manifest binding after run event was accepted")
	}
}

func TestRunManifestRejectsRawCandidateArtifact(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "candidate-prep")
	operation, _ := Open(root, "candidate-exp")
	_, _ = operation.Begin("candidate-prep")
	inputs := testRunInputs(t, operation, "run-1", "candidate")
	inputs.Candidate = putTestArtifact(t, operation, "raw candidate bytes")
	if _, err := RecordPreparedRun(operation.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: "run-1", Inputs: inputs}); err == nil {
		t.Fatal("run manifest accepted a raw candidate artifact")
	}
}

func TestRunManifestRequiresExactFixtureResetProof(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "reset-prep")
	operation, _ := Open(root, "reset-exp")
	_, _ = operation.Begin("reset-prep")
	inputs := testRunInputs(t, operation, "other-run", "reset")
	if _, err := RecordPreparedRun(operation.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: "run-1", Inputs: inputs}); err == nil {
		t.Fatal("fixture reset proof for another run was accepted")
	}
	inputs.FixtureReset = recordTestFixtureReset(t, operation, "run-1", putTestArtifact(t, operation, "other-fixture"), putTestArtifact(t, operation, "baseline"))
	if _, err := RecordPreparedRun(operation.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: "run-1", Inputs: inputs}); err == nil {
		t.Fatal("fixture reset proof for another fixture was accepted")
	}
}

func TestRunManifestHardCutsLegacyContract(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "legacy-prep")
	op, _ := Open(root, "legacy-exp")
	_, _ = op.Begin("legacy-prep")
	inputs := testRunInputs(t, op, "run-1", "legacy")
	current, err := op.current()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(RunManifest{
		Contract: "agentlab.run-manifest.v1", WorkerInput: current.begun.WorkerInput, SourceSnapshot: current.begun.Source,
		Origin: NewFreshOrigin(), RunInputs: inputs,
	})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.artifacts.Put(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.readManifest(ref); err == nil {
		t.Fatal("legacy run manifest contract was accepted")
	}
}

func testRunInputs(t *testing.T, operation *Operation, runID, prefix string) RunInputs {
	fixture := putTestArtifact(t, operation, prefix+"-fixture")
	baseline := putTestArtifact(t, operation, prefix+"-fixture-baseline")
	return RunInputs{
		Harness: putTestArtifact(t, operation, prefix+"-harness"), Trial: putTestArtifact(t, operation, prefix+"-trial"),
		Candidate: testCandidate(t, operation, prefix+"-candidate"), Adapter: putTestArtifact(t, operation, prefix+"-adapter"),
		OracleSet: putTestArtifact(t, operation, prefix+"-oracles"), Fixture: fixture,
		FixtureReset:   recordTestFixtureReset(t, operation, runID, fixture, baseline),
		EvidencePolicy: putTestArtifact(t, operation, prefix+"-evidence"), StopPolicy: putTestArtifact(t, operation, prefix+"-stop"),
		WorkerRuntime: putTestArtifact(t, operation, prefix+"-runtime"), Environment: putTestArtifact(t, operation, prefix+"-environment"),
	}
}

func putTestArtifact(t *testing.T, operation *Operation, value string) artifact.Ref {
	t.Helper()
	ref, err := operation.artifacts.Put([]byte(value))
	if err != nil {
		t.Fatal(err)
	}
	return ref
}
