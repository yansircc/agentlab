package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
)

func (o *Operation) Compare(observation comparison.Observation) (comparison.Result, error) {
	if err := observation.Validate(); err != nil {
		return comparison.Result{}, err
	}
	current, err := o.current()
	if err != nil {
		return comparison.Result{}, err
	}
	if current.comparisons[observation.ID].ID != "" {
		return comparison.Result{}, errors.New("comparison id already exists")
	}
	candidate := current.candidates[observation.CandidateID]
	if candidate.ID == "" {
		return comparison.Result{}, errors.New("comparison candidate does not exist")
	}
	if !diagnosisOwnsClaims(current.diagnoses[candidate.DiagnosisID], observation.Policy.RequiredClaims) {
		return comparison.Result{}, errors.New("comparison claims are not owned by candidate diagnosis")
	}
	manifests, err := o.comparisonManifests(append(append([]string(nil), observation.BaselineRuns...), observation.CandidateRuns...))
	if err != nil {
		return comparison.Result{}, err
	}
	result, err := comparison.Evaluate(observation, manifests, candidate.Artifact)
	if err != nil {
		return comparison.Result{}, err
	}
	err = o.mutate(func(current *state) error {
		if current.comparisons[observation.ID].ID != "" {
			return errors.New("comparison id already exists")
		}
		return o.append(eventComparison, observation)
	})
	return result, err
}

func (o *Operation) Comparison(id string) (comparison.Result, error) {
	current, err := o.current()
	if err != nil {
		return comparison.Result{}, err
	}
	observation := current.comparisons[id]
	if observation.ID == "" {
		return comparison.Result{}, errors.New("comparison does not exist")
	}
	candidate := current.candidates[observation.CandidateID]
	if !diagnosisOwnsClaims(current.diagnoses[candidate.DiagnosisID], observation.Policy.RequiredClaims) {
		return comparison.Result{}, errors.New("comparison claims are not owned by candidate diagnosis")
	}
	manifests, err := o.comparisonManifests(append(append([]string(nil), observation.BaselineRuns...), observation.CandidateRuns...))
	if err != nil {
		return comparison.Result{}, err
	}
	return comparison.Evaluate(observation, manifests, candidate.Artifact)
}

func (o *Operation) comparisonManifests(ids []string) (map[string]comparison.RunIdentity, error) {
	result := make(map[string]comparison.RunIdentity, len(ids))
	for _, id := range ids {
		manifest, _, err := o.RunManifest(id)
		if err != nil {
			return nil, err
		}
		reset, err := loadFixtureReset(o.artifacts, manifest.FixtureReset)
		if err != nil || reset.RunID != id || reset.Fixture != manifest.Fixture {
			return nil, errors.New("run fixture reset proof is invalid")
		}
		result[id] = comparison.RunIdentity{
			RunID: id, WorkerInput: manifest.WorkerInput, Harness: manifest.Harness, Trial: manifest.Trial,
			Candidate: manifest.Candidate, Adapter: manifest.Adapter, OracleSet: manifest.OracleSet,
			Fixture: manifest.Fixture, FixtureReset: manifest.FixtureReset, FixtureBaseline: reset.Baseline, EvidencePolicy: manifest.EvidencePolicy,
			StopPolicy: manifest.StopPolicy, WorkerRuntime: manifest.WorkerRuntime,
			Environment: manifest.Environment,
		}
	}
	return result, nil
}

func diagnosisOwnsClaims(value diagnosis.Diagnosis, required []string) bool {
	owned := map[string]bool{}
	for _, claim := range value.AcceptanceClaims {
		owned[claim.ID] = true
	}
	for _, id := range required {
		if !owned[id] {
			return false
		}
	}
	return true
}
