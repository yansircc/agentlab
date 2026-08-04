package deployctlfixture

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const deployctlTrialContract = "agentlab.deployctl-trial.v1"

type deployctlTrial struct {
	Contract string `json:"contract"`
	Release  string `json:"release"`
	Target   string `json:"target"`
}

func loadDeployctlTrial(store artifact.Store, ref artifact.Ref) (deployctlTrial, error) {
	data, err := store.Read(ref)
	if err != nil {
		return deployctlTrial{}, err
	}
	var value deployctlTrial
	if strictjson.Decode(data, &value) != nil || value.Contract != deployctlTrialContract || !validTarget(value.Target) || !validTarget(value.Release) {
		return deployctlTrial{}, errors.New("deployctl trial is invalid")
	}
	return value, nil
}

func preflightInputs(store artifact.Store, runID string, reset ResetReceipt, candidate, adapter, runtime artifact.Ref) (experiment.RunInputs, artifact.Ref, error) {
	fixtureRef, err := putCanonical(store, struct {
		Contract string `json:"contract"`
		Catalog  string `json:"catalog_digest"`
	}{Contract: "agentlab.deployctl-fixture.v1", Catalog: reset.CatalogDigest})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	baseline, err := putCanonical(store, struct {
		Contract string `json:"contract"`
		State    string `json:"state_digest"`
	}{Contract: "agentlab.deployctl-baseline.v1", State: reset.StateDigest})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	resetEvidence, err := putCanonical(store, reset)
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	resetRef, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{
		Contract: experiment.FixtureResetContract, RunID: runID, Fixture: fixtureRef, Baseline: baseline, Evidence: []artifact.Ref{resetEvidence},
	})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	harness, err := putCanonical(store, map[string]string{"contract": "agentlab.deployctl-harness.v1", "task": "public-cli-deployment"})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	trial, err := putCanonical(store, deployctlTrial{Contract: deployctlTrialContract, Release: "release-a", Target: "staging"})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	oracles, err := putCanonical(store, map[string]any{"contract": "agentlab.deployctl-oracles.v1", "claims": []string{"staging-release", "production-unchanged", "receipt-target", "write-set", "default-target-read-count"}})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	evidence, err := putCanonical(store, map[string]string{"contract": "agentlab.deployctl-evidence-policy.v1", "visibility": "public-only"})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	stop, err := putCanonical(store, map[string]string{"contract": "agentlab.deployctl-stop-policy.v1", "material_failure": "target-receipt-or-production-mismatch"})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	environment, err := putCanonical(store, map[string]string{"contract": "agentlab.deployctl-environment.v1", "scope": "disposable-fixture"})
	if err != nil {
		return experiment.RunInputs{}, artifact.Ref{}, err
	}
	return experiment.RunInputs{
		Harness: harness, Trial: trial, Candidate: candidate, Adapter: adapter, OracleSet: oracles, Fixture: fixtureRef, FixtureReset: resetRef,
		EvidencePolicy: evidence, StopPolicy: stop, WorkerRuntime: runtime, Environment: environment,
	}, resetRef, nil
}
