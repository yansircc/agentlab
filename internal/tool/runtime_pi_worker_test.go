package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
)

func TestWorkerPromptFreshRunContainsOnlySealedWorkerInput(t *testing.T) {
	binding, _, _, launch := workerPromptFixture(t)
	prompt, err := workerPrompt(binding, "worker", launch)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := preparation.RenderWorkerInput(binding.store(), launch.WorkerInput)
	if err != nil {
		t.Fatal(err)
	}
	want := "You are the isolated Worker. Use only the public deployctl tools to complete the sealed task.\n\n" + sealed
	if prompt != want {
		t.Fatalf("fresh Worker prompt = %q, want %q", prompt, want)
	}
}

func TestWorkerPromptAppendsExactOwnedSpliceIntervention(t *testing.T) {
	binding, experimentOp, parentEvidence, launch := workerPromptFixture(t)
	parent, err := run.Open(binding.Root, binding.ExperimentID, "worker")
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := recordDecisionBoundTestCheckpoint(t, binding, parent, parentEvidence, "checkpoint-worker", run.RuntimeCheckpointSpec{
		Adapter: "test", Session: []byte("parent-session"), OpaqueState: []byte("parent-state"), PublicPrefix: []byte("parent-public-prefix"),
	})
	text := "The public deployment contract changed. Re-observe target, status, and receipt before continuing."
	intervention, err := experimentOp.RecordInterventionWithDecision(experiment.DecisionBoundIntervention{
		Decision: experiment.SupervisorDecision{
			ID: "record-intervention", WorkerRun: "worker", EvidenceThrough: parentEvidence.Sequence,
			Claim: "the public deployment contract changed", Action: experiment.DecisionIntervention,
			Evidence: []run.EvidenceRef{parentEvidence}, Falsifier: "the public contract remains unchanged",
		},
		Intervention: experiment.Intervention{Contract: experiment.InterventionContract, Text: text},
	})
	if err != nil {
		t.Fatal(err)
	}
	parentManifest, _, err := experimentOp.RunManifest("worker")
	if err != nil {
		t.Fatal(err)
	}
	inputs := parentManifest.RunInputs
	fixture, err := binding.store().Put([]byte("child-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := binding.store().Put([]byte("child-fixture-baseline"))
	if err != nil {
		t.Fatal(err)
	}
	resetEvidence, err := binding.store().Put([]byte("child-reset-evidence"))
	if err != nil {
		t.Fatal(err)
	}
	reset, err := experiment.RecordFixtureReset(binding.store(), experiment.FixtureResetProof{
		Contract: experiment.FixtureResetContract, RunID: "child", Fixture: fixture, Baseline: baseline, Evidence: []artifact.Ref{resetEvidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	inputs.Fixture, inputs.FixtureReset = fixture, reset
	prepared, err := experiment.RecordPreparedRun(binding.store(), experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: "child", Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	origin, err := experiment.NewSpliceOrigin(experiment.SpliceOriginSpec{
		ParentRun: "worker", ParentEvidence: parentEvidence, RuntimeCheckpoint: checkpoint.Checkpoint, PublicPrefix: checkpoint.PublicPrefix,
		Intervention: &intervention, ReasonEvidence: []run.EvidenceRef{parentEvidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := experimentOp.BindPreparedRun("child", origin, prepared); err != nil {
		t.Fatal(err)
	}
	prompt, err := workerPrompt(binding, "child", launch)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := preparation.RenderWorkerInput(binding.store(), launch.WorkerInput)
	if err != nil {
		t.Fatal(err)
	}
	want := "You are the isolated Worker. Use only the public deployctl tools to complete the sealed task.\n\n" + sealed + "\n\nIntervention:\n" + text
	if prompt != want {
		t.Fatalf("splice Worker prompt = %q, want %q", prompt, want)
	}
}

func recordDecisionBoundTestCheckpoint(t *testing.T, binding Binding, parent *run.Operation, evidence run.EvidenceRef, id string, spec run.RuntimeCheckpointSpec) run.RuntimeCheckpointRecord {
	t.Helper()
	experimentOp, err := binding.experiment()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := binding.store().Put([]byte(id + " payload"))
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: id, RunID: evidence.RunID, Kind: effect.Checkpoint, Payload: payload}
	decision := experiment.SupervisorDecision{ID: id, WorkerRun: evidence.RunID, EvidenceThrough: evidence.Sequence, Claim: "preserve the selected public checkpoint", Action: experiment.DecisionCheckpoint, Evidence: []run.EvidenceRef{evidence}, Falsifier: "the checkpoint is not admissible"}
	if err := experimentOp.CommitDecisionBoundEffect(experiment.DecisionBoundEffect{Decision: decision, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := parent.RecordRuntimeCheckpoint(intent, spec)
	if err != nil {
		t.Fatal(err)
	}
	receipt := []byte(id + " receipt")
	if err := parent.RecordEffectObservation(intent, receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := parent.SettleEffect(intent, receipt); err != nil {
		t.Fatal(err)
	}
	return checkpoint
}

func workerPromptFixture(t *testing.T) (Binding, *experiment.Operation, run.EvidenceRef, PiWorkerLaunch) {
	t.Helper()
	root := t.TempDir()
	binding := Binding{Root: root, PreparationID: "prep", ExperimentID: "exp"}
	_ = renderOwnedHandoff(t, binding)
	experimentOp, err := binding.experiment()
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := experimentOp.RunManifest("worker")
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "deployctl")
	bytes := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(executable, bytes, 0o700); err != nil {
		t.Fatal(err)
	}
	stored, err := binding.store().Put(bytes)
	if err != nil {
		t.Fatal(err)
	}
	candidateExecutable, err := run.BindCandidateExecutable(binding.store(), manifest.Candidate, stored)
	if err != nil {
		t.Fatal(err)
	}
	launch := PiWorkerLaunch{DeployctlExecutable: executable, CandidateExecutable: candidateExecutable, WorkerInput: manifest.WorkerInput}
	evidence := run.EvidenceRef{ExperimentID: binding.ExperimentID, RunID: "worker", Sequence: 2, Item: 0}
	return binding, experimentOp, evidence, launch
}
