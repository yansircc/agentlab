package experiment

import (
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
	first, err := operation.BindRun("run-1", inputs)
	if err != nil {
		t.Fatal(err)
	}
	if same, err := operation.BindRun("run-1", inputs); err != nil || same != first {
		t.Fatalf("exact binding was not idempotent: %#v, %v", same, err)
	}
	changed := inputs
	changed.Trial = putTestArtifact(t, operation, "changed-trial")
	if _, err := operation.BindRun("run-1", changed); err == nil {
		t.Fatal("changed manifest was accepted for bound run")
	}
}

func TestRunManifestCannotBindAfterRunEvent(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "event-prep")
	operation, _ := Open(root, "event-exp")
	_, _ = operation.Begin("event-prep")
	inputs := testRunInputs(t, operation, "run-1", "bound")
	if _, err := operation.BindRun("run-1", inputs); err != nil {
		t.Fatal(err)
	}
	runOperation, _ := run.Open(root, "event-exp", "run-1")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := runOperation.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	if _, err := operation.BindRun("run-1", inputs); err == nil {
		t.Fatal("manifest binding after run event was accepted")
	}
}

func TestRunManifestRequiresExactFixtureResetProof(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "reset-prep")
	operation, _ := Open(root, "reset-exp")
	_, _ = operation.Begin("reset-prep")
	inputs := testRunInputs(t, operation, "other-run", "reset")
	if _, err := operation.BindRun("run-1", inputs); err == nil {
		t.Fatal("fixture reset proof for another run was accepted")
	}
	inputs.FixtureReset = recordTestFixtureReset(t, operation, "run-1", putTestArtifact(t, operation, "other-fixture"), putTestArtifact(t, operation, "baseline"))
	if _, err := operation.BindRun("run-1", inputs); err == nil {
		t.Fatal("fixture reset proof for another fixture was accepted")
	}
}

func testRunInputs(t *testing.T, operation *Operation, runID, prefix string) RunInputs {
	fixture := putTestArtifact(t, operation, prefix+"-fixture")
	baseline := putTestArtifact(t, operation, prefix+"-fixture-baseline")
	return RunInputs{
		Harness: putTestArtifact(t, operation, prefix+"-harness"), Trial: putTestArtifact(t, operation, prefix+"-trial"),
		Candidate: putTestArtifact(t, operation, prefix+"-candidate"), Adapter: putTestArtifact(t, operation, prefix+"-adapter"),
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
