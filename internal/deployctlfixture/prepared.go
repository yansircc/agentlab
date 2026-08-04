package deployctlfixture

import (
	"errors"
	"path/filepath"
	"regexp"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/tool"
	"github.com/yansircc/agentlab/internal/transaction"
)

var preparedRunID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// PrepareBaselineRun is the Host producer for an additional clean baseline
// repetition. It has no caller-supplied candidate: the baseline candidate is
// recovered from the Host-verified preflight and bound into one complete
// PreparedRun with its own fixture, executable, and runtime profile.
func (value Preflight) PrepareBaselineRun(runID string) (artifact.Ref, error) {
	return value.prepareFreshRun(runID, value.Candidate)
}

// PrepareRunFromCoderCompletion is the Host producer for a future Worker
// manifest. It derives the candidate solely from the exact terminal Coder
// completion, then issues one complete PreparedRun artifact. It never accepts
// provider-supplied RunInputs or candidate refs.
func (value Preflight) PrepareRunFromCoderCompletion(runID string, completionRef artifact.Ref) (artifact.Ref, error) {
	if !completionRef.Valid() {
		return artifact.Ref{}, errors.New("deployctl prepared run request is invalid")
	}
	completion, err := value.terminalCoderCompletion(completionRef)
	if err != nil {
		return artifact.Ref{}, err
	}
	return value.prepareFreshRun(runID, completion.Candidate)
}

// prepareFreshRun owns all changeable run inputs. Its candidate is reachable
// only from a Host-verified preflight baseline or terminal Coder completion;
// the provider boundary never supplies it or any individual RunInputs field.
func (value Preflight) prepareFreshRun(runID string, candidate artifact.Ref) (artifact.Ref, error) {
	if !preparedRunID.MatchString(runID) || runID == baselineRunID || runID == coderRunID || !candidate.Valid() {
		return artifact.Ref{}, errors.New("deployctl prepared run request is invalid")
	}
	if err := value.verifyRuntime(); err != nil {
		return artifact.Ref{}, errors.New("deployctl runtime preflight is unavailable")
	}
	lease, err := transaction.Acquire(filepath.Join(value.hostRoot, "prepared-run-producer.lock"))
	if err != nil {
		return artifact.Ref{}, err
	}
	defer lease.Release()

	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		return artifact.Ref{}, err
	}
	status, err := op.Status()
	if err != nil {
		return artifact.Ref{}, err
	}
	for _, existing := range status.RunIDs {
		if existing == runID {
			return artifact.Ref{}, errors.New("deployctl prepared run is already bound")
		}
	}

	fixture, err := New(filepath.Join(value.EvaluatedRoot, "candidate-fixture-"+runID))
	if err != nil {
		return artifact.Ref{}, err
	}
	reset, err := fixture.Reset()
	if err != nil {
		return artifact.Ref{}, err
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	workspace := filepath.Join(value.hostRoot, "candidate-workspace-"+runID)
	executable := filepath.Join(workspace, "bin", "deployctl")
	candidateExecutable, err := BuildCandidate(store, candidate, workspace, executable)
	if err != nil {
		return artifact.Ref{}, err
	}
	runtime, err := recordRuntimeBinding(store, runtimeBinding{
		Contract: runtimeBindingContract, Adapter: value.LiveCanary, CandidateExecutable: candidateExecutable,
		WorkerProfile: candidateWorkerProfileRef(runID), CoderProfile: "coder-repair",
	})
	if err != nil {
		return artifact.Ref{}, err
	}
	inputs, _, err := preflightInputs(store, runID, reset, candidate, value.LiveCanary, runtime)
	if err != nil {
		return artifact.Ref{}, err
	}
	prepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{
		Contract: experiment.PreparedRunContract, RunID: runID, Inputs: inputs,
	})
	if err != nil {
		return artifact.Ref{}, err
	}
	profile, err := value.candidateWorkerRuntime(runID, runtime, fixture, executable, candidateExecutable)
	if err != nil {
		return artifact.Ref{}, err
	}
	if err := tool.AppendPiPreparedWorkerRuntime(value.runtimePlanPath, profile); err != nil {
		return artifact.Ref{}, err
	}
	return prepared, nil
}

func (value Preflight) terminalCoderCompletion(receipt artifact.Ref) (run.CoderCompletion, error) {
	coder, err := run.Open(value.EvaluatedRoot, value.ExperimentID, coderRunID)
	if err != nil {
		return run.CoderCompletion{}, err
	}
	actual, completion, err := coder.CoderCompletionReceipt()
	if err != nil || actual != receipt {
		return run.CoderCompletion{}, errors.New("deployctl Coder completion is not terminal")
	}
	host, err := tool.LoadPiRuntimeHost(value.runtimePlanPath)
	if err != nil {
		return run.CoderCompletion{}, errors.New("deployctl runtime plan is invalid")
	}
	profile, err := host.Profile("coder-repair")
	if err != nil {
		return run.CoderCompletion{}, errors.New("deployctl Coder profile is absent")
	}
	want := run.CoderProfile{
		SourceSnapshot: profile.CoderSourceSnapshot, CandidateWorkspace: profile.CoderWorkspaceReceipt,
		CapabilityProfile: profile.CoderCapabilityProfile,
	}
	if profile.Role != effect.CoderStart || profile.RunID != coderRunID || completion.Profile.SourceSnapshot != want.SourceSnapshot || completion.Profile.CandidateWorkspace != want.CandidateWorkspace || completion.Profile.CapabilityProfile != want.CapabilityProfile {
		return run.CoderCompletion{}, errors.New("deployctl Coder completion differs from Host profile")
	}
	return completion, nil
}

func (value Preflight) candidateWorkerRuntime(runID string, runtime artifact.Ref, fixture Fixture, executable string, candidateExecutable artifact.Ref) (tool.PiPreparedWorkerRuntime, error) {
	host, err := tool.LoadPiRuntimeHost(value.runtimePlanPath)
	if err != nil {
		return tool.PiPreparedWorkerRuntime{}, errors.New("deployctl runtime plan is invalid")
	}
	profile, err := host.Profile("baseline-worker")
	if err != nil || profile.WorkerLaunch == nil {
		return tool.PiPreparedWorkerRuntime{}, errors.New("deployctl Worker profile is absent")
	}
	launch := *profile.WorkerLaunch
	launchConfig := launch.Launch
	runtimeRoot := filepath.Join(value.hostRoot, "candidate-runtime-"+runID)
	launchConfig.RuntimeRoot = runtimeRoot
	launch.Launch = launchConfig
	launch.FixtureRoot = fixture.Root()
	launch.DeployctlExecutable = executable
	launch.CandidateExecutable = candidateExecutable
	template := tool.PiPreparedWorkerRuntime{
		Ref: candidateWorkerProfileRef(runID), ExperimentID: value.ExperimentID, RunID: runID, WorkerRuntime: runtime,
		FreshSessionPath: filepath.Join(runtimeRoot, "session.jsonl"), Identity: profile.Identity,
		Policy:       profile.Policy,
		WorkerLaunch: launch,
	}
	if template.Validate() != nil {
		return tool.PiPreparedWorkerRuntime{}, errors.New("deployctl candidate Worker runtime is invalid")
	}
	return template, nil
}
