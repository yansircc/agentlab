package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func (o *Operation) Compare(observation comparison.Observation) (comparison.Result, error) {
	return o.compare(observation, nil)
}

func (o *Operation) compare(observation comparison.Observation, decision *SupervisorDecision) (comparison.Result, error) {
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
		if current.comparisons[observation.ID].ID != "" || (decision != nil && current.decisions[decision.ID].ID != "") {
			return errors.New("comparison id already exists")
		}
		if decision != nil {
			return o.append(eventDecisionComparison, DecisionBoundComparison{Decision: *decision, Observation: observation})
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
	current, err := o.current()
	if err != nil {
		return nil, err
	}
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
		operation, err := run.Open(o.root, o.id, id)
		if err != nil {
			return nil, err
		}
		if err := o.verifyComparisonWorkerStart(current, id, operation); err != nil {
			return nil, err
		}
		accepted, err := operation.TerminalAccepted()
		if err != nil || !accepted {
			return nil, errors.New("comparison run has no accepted terminal result")
		}
		oracleRef, claims, err := o.comparisonOracleEvidence(id, manifest, operation)
		if err != nil {
			return nil, err
		}
		result[id] = comparison.RunIdentity{
			RunID: id, Origin: comparisonOrigin(manifest.Origin), Intervention: hasIntervention(manifest.Origin), WorkerInput: manifest.WorkerInput, Harness: manifest.Harness, Trial: manifest.Trial,
			Candidate: manifest.Candidate, Adapter: manifest.Adapter, OracleSet: manifest.OracleSet,
			Fixture: manifest.Fixture, FixtureReset: manifest.FixtureReset, FixtureBaseline: reset.Baseline, EvidencePolicy: manifest.EvidencePolicy,
			StopPolicy: manifest.StopPolicy, WorkerRuntime: manifest.WorkerRuntime,
			Environment: manifest.Environment, StartVerified: true, TerminalAccepted: true, OracleEvidence: oracleRef, OracleClaims: claims,
		}
	}
	return result, nil
}

func (o *Operation) verifyComparisonWorkerStart(current state, runID string, operation *run.Operation) error {
	var start *DecisionBoundEffect
	for _, id := range current.effectOrder {
		value := current.effects[id]
		if value.Intent.RunID != runID || value.Intent.Kind != effect.WorkerStart {
			continue
		}
		if start != nil {
			return errors.New("comparison run has multiple decision-bound Worker starts")
		}
		copy := value
		start = &copy
	}
	if start == nil {
		return errors.New("comparison run has no decision-bound Worker start")
	}
	decisionAt, err := o.decisionBoundEffectTime(start.Intent.ID)
	if err != nil {
		return err
	}
	records, err := operation.Inspect(0, 1)
	if err != nil || len(records) != 1 || records[0].Kind != "process_started" || decisionAt.After(records[0].At) {
		return errors.New("comparison Worker start is not temporally verified")
	}
	if err := operation.VerifyStartEffect(start.Intent); err != nil {
		return errors.New("comparison Worker start is not verified")
	}
	return nil
}

func (o *Operation) comparisonOracleEvidence(runID string, manifest RunManifest, operation *run.Operation) (artifact.Ref, []comparison.OracleClaim, error) {
	items, err := operation.OracleEvidence()
	if err != nil {
		return artifact.Ref{}, nil, err
	}
	if len(items) != 1 || !items[0].Raw.Valid() {
		return artifact.Ref{}, nil, errors.New("comparison run requires one objective oracle evidence artifact")
	}
	value, err := comparison.LoadOracleEvidence(o.artifacts, items[0].Raw)
	if err != nil || value.RunID != runID || value.Candidate != manifest.Candidate || value.Trial != manifest.Trial || value.OracleSet != manifest.OracleSet {
		return artifact.Ref{}, nil, errors.New("comparison oracle evidence differs from run manifest")
	}
	return items[0].Raw, append([]comparison.OracleClaim(nil), value.Claims...), nil
}

func comparisonOrigin(origin RunOrigin) comparison.Origin {
	if origin.IsFresh() {
		return comparison.FreshOrigin
	}
	return comparison.SpliceOrigin
}

func hasIntervention(origin RunOrigin) bool {
	splice, ok := origin.Splice()
	return ok && splice.Intervention != nil
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
