package deployctlfixture

import (
	"errors"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/tool"
)

// RecordWorkerOracle is a Host-only objective observation for one active
// deployctl Worker run. It derives every result from the exact Host runtime
// profile and immutable manifest, never from Worker text or Supervisor input.
func (value Preflight) RecordWorkerOracle(runID string) (run.EvidenceRef, error) {
	if !preparedRunID.MatchString(runID) || value.verifyRuntime() != nil {
		return run.EvidenceRef{}, errors.New("deployctl Worker oracle preflight is unavailable")
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	experimentOp, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	manifest, _, err := experimentOp.RunManifest(runID)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	fixture, err := value.oracleFixture(store, runID, manifest)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	trial, err := loadDeployctlTrial(store, manifest.Trial)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	result, err := fixture.Oracle(trial.Target, trial.Release)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	receipt, receiptOK := fixture.receipt()
	receiptTarget := receiptOK && receipt.Target == trial.Target && receipt.Release == trial.Release && result.ReceiptMatchesActual
	evidence, err := comparison.EncodeOracleEvidence(comparison.OracleEvidence{
		Contract: comparison.OracleEvidenceContract, RunID: runID, Candidate: manifest.Candidate, Trial: manifest.Trial, OracleSet: manifest.OracleSet,
		Claims: []comparison.OracleClaim{
			{ID: "staging-release", Passed: result.TargetReleaseMatches, HeldOut: true},
			{ID: "production-unchanged", Passed: result.ProductionUnchanged, HeldOut: true},
			{ID: "receipt-target", Passed: receiptTarget, HeldOut: true},
			{ID: "write-set", Passed: len(result.WriteSet) == 1 && result.WriteSet[0] == trial.Target, HeldOut: true},
			{ID: "default-target-read-count", Passed: result.DefaultTargetReadCount == 0, HeldOut: true},
		},
	})
	if err != nil {
		return run.EvidenceRef{}, err
	}
	raw, err := store.Put(evidence)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	worker, err := run.Open(value.EvaluatedRoot, value.ExperimentID, runID)
	if err != nil {
		return run.EvidenceRef{}, err
	}
	return worker.RecordHostOracleEvidence(raw)
}

func (value Preflight) oracleFixture(store artifact.Store, runID string, manifest experiment.RunManifest) (Fixture, error) {
	binding, err := loadRuntimeBinding(store, manifest.WorkerRuntime)
	if err != nil {
		return Fixture{}, err
	}
	host, err := tool.LoadPiRuntimeHost(value.runtimePlanPath)
	if err != nil {
		return Fixture{}, err
	}
	launch, err := oracleWorkerLaunch(host, binding.WorkerProfile, runID)
	if err != nil || launch.CandidateExecutable != binding.CandidateExecutable || run.VerifyCandidateExecutable(store, launch.CandidateExecutable, manifest.Candidate, launch.DeployctlExecutable) != nil {
		return Fixture{}, errors.New("deployctl Worker oracle differs from Host runtime")
	}
	return OpenFixture(launch.FixtureRoot)
}

func oracleWorkerLaunch(host *tool.PiRuntimeHost, ref, runID string) (tool.PiWorkerLaunch, error) {
	if profile, err := host.Profile(ref); err == nil {
		if profile.RunID != runID || profile.WorkerLaunch == nil {
			return tool.PiWorkerLaunch{}, errors.New("deployctl Worker runtime profile is invalid")
		}
		return *profile.WorkerLaunch, nil
	}
	prepared, err := host.PreparedWorker(ref)
	if err != nil || prepared.RunID != runID {
		return tool.PiWorkerLaunch{}, errors.New("deployctl prepared Worker runtime is invalid")
	}
	return prepared.WorkerLaunch, nil
}
