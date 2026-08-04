package comparison

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/yansircc/agentlab/internal/artifact"
)

func Evaluate(observation Observation, manifests map[string]RunIdentity, candidate artifact.Ref) (Result, error) {
	if err := observation.Validate(); err != nil {
		return Result{}, err
	}
	baseline, err := identities(observation.BaselineRuns, manifests)
	if err != nil {
		return Result{}, err
	}
	candidateRuns, err := identities(observation.CandidateRuns, manifests)
	if err != nil {
		return Result{}, err
	}
	reasons := validateControlledInputs(baseline, candidateRuns, candidate)
	for _, fact := range observation.ValidityFacts {
		if !fact.Valid {
			reasons = append(reasons, "invalid validity fact: "+fact.Kind)
		}
	}
	if len(reasons) != 0 {
		return Result{Observation: observation, Verdict: Invalid, Reasons: reasons}, nil
	}
	deltas, err := requiredDeltas(observation.Policy.RequiredClaims, observation.ClaimDeltas)
	if err != nil {
		return Result{}, err
	}
	if len(baseline) < observation.Policy.MinimumRepetitions || len(candidateRuns) < observation.Policy.MinimumRepetitions {
		return Result{Observation: observation, Verdict: Inconclusive, Reasons: []string{"repetition threshold not met"}}, nil
	}
	for _, delta := range deltas {
		if delta.CandidateFailures > delta.BaselineFailures {
			return Result{Observation: observation, Verdict: SupportedRegression, Reasons: []string{"candidate regressed required claim " + delta.ClaimID}}, nil
		}
	}
	for _, delta := range observation.ClaimDeltas {
		if delta.HeldOut && delta.CandidateFailures > delta.BaselineFailures {
			return Result{Observation: observation, Verdict: SupportedRegression, Reasons: []string{"candidate regressed held-out claim " + delta.ClaimID}}, nil
		}
	}
	improved := false
	for _, delta := range deltas {
		if delta.CandidateFailures != 0 {
			return Result{Observation: observation, Verdict: Inconclusive, Reasons: []string{"candidate still fails required claim " + delta.ClaimID}}, nil
		}
		improved = improved || delta.BaselineFailures > delta.CandidateFailures
	}
	if improved {
		return Result{Observation: observation, Verdict: SupportedImprovement}, nil
	}
	return Result{Observation: observation, Verdict: Equivalent}, nil
}

func identities(ids []string, manifests map[string]RunIdentity) ([]RunIdentity, error) {
	result := make([]RunIdentity, 0, len(ids))
	for _, id := range ids {
		identity, ok := manifests[id]
		if !ok || identity.RunID != id || !validIdentity(identity) {
			return nil, errors.New("comparison run manifest is absent: " + id)
		}
		result = append(result, identity)
	}
	return result, nil
}

func validIdentity(value RunIdentity) bool {
	if value.Origin != FreshOrigin && value.Origin != SpliceOrigin {
		return false
	}
	refs := []artifact.Ref{value.WorkerInput, value.Harness, value.Trial, value.Candidate, value.Adapter, value.OracleSet, value.Fixture, value.FixtureReset, value.FixtureBaseline, value.EvidencePolicy, value.StopPolicy, value.WorkerRuntime, value.Environment}
	for _, ref := range refs {
		if !ref.Valid() {
			return false
		}
	}
	return true
}

func validateControlledInputs(baseline, candidate []RunIdentity, candidateArtifact artifact.Ref) []string {
	var reasons []string
	all := append(append([]RunIdentity(nil), baseline...), candidate...)
	stable := all[0]
	for _, current := range all {
		if current.Origin != FreshOrigin || current.Intervention {
			reasons = append(reasons, "guided or intervened run cannot support autonomous comparison")
			break
		}
	}
	for _, current := range all[1:] {
		// WorkerRuntime binds the exact Host profile and executable for one
		// isolated run. It necessarily changes with its candidate, fixture,
		// session, and run id, so PreparedRun is its single owner rather than
		// a cross-run equality check. Adapter and Environment remain controls.
		if current.WorkerInput != stable.WorkerInput || current.Harness != stable.Harness || current.Adapter != stable.Adapter || current.OracleSet != stable.OracleSet || current.Fixture != stable.Fixture || current.FixtureBaseline != stable.FixtureBaseline || current.EvidencePolicy != stable.EvidencePolicy || current.StopPolicy != stable.StopPolicy || current.Environment != stable.Environment {
			reasons = append(reasons, "controlled run inputs differ")
			break
		}
	}
	if baseline[0].Candidate == candidateArtifact {
		reasons = append(reasons, "baseline is already bound to candidate")
	}
	for _, current := range baseline[1:] {
		if current.Candidate != baseline[0].Candidate {
			reasons = append(reasons, "baseline candidate identities differ")
			break
		}
	}
	if trialSet(baseline) != trialSet(candidate) {
		reasons = append(reasons, "trial multisets differ")
	}
	for _, current := range candidate {
		if current.Candidate != candidateArtifact {
			reasons = append(reasons, "candidate run is not bound to exact candidate")
			break
		}
	}
	return reasons
}

func trialSet(values []RunIdentity) string {
	digests := make([]string, 0, len(values))
	for _, value := range values {
		digests = append(digests, value.Trial.Algorithm+":"+value.Trial.Digest)
	}
	sort.Strings(digests)
	data, _ := json.Marshal(digests)
	return string(data)
}

func requiredDeltas(required []string, values []ClaimDelta) ([]ClaimDelta, error) {
	byID := map[string]ClaimDelta{}
	for _, value := range values {
		if _, exists := byID[value.ClaimID]; exists {
			return nil, errors.New("claim delta is duplicated")
		}
		byID[value.ClaimID] = value
	}
	result := make([]ClaimDelta, 0, len(required))
	for _, id := range required {
		value, ok := byID[id]
		if !ok {
			return nil, errors.New("required claim delta is absent: " + id)
		}
		result = append(result, value)
	}
	return result, nil
}
