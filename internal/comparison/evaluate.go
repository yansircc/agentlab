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
	reasons = append(reasons, validateRunEvidence(append(append([]RunIdentity(nil), baseline...), candidateRuns...))...)
	if len(reasons) != 0 {
		return Result{Observation: observation, Verdict: Invalid, Reasons: reasons}, nil
	}
	deltas, err := derivedDeltas(observation.Policy, baseline, candidateRuns)
	if err != nil {
		return Result{}, err
	}
	if len(baseline) < observation.Policy.MinimumRepetitions || len(candidateRuns) < observation.Policy.MinimumRepetitions {
		return Result{Observation: observation, Verdict: Inconclusive, Reasons: []string{"repetition threshold not met"}}, nil
	}
	for _, delta := range deltas {
		if delta.CandidateFailures > delta.BaselineFailures && (delta.Required || delta.HeldOut) {
			kind := "required"
			if delta.HeldOut && !delta.Required {
				kind = "held-out"
			}
			return Result{Observation: observation, Verdict: SupportedRegression, Reasons: []string{"candidate regressed " + kind + " claim " + delta.ID}}, nil
		}
	}
	improved := false
	for _, delta := range deltas {
		if !delta.Required {
			continue
		}
		if delta.CandidateFailures != 0 {
			return Result{Observation: observation, Verdict: Inconclusive, Reasons: []string{"candidate still fails required claim " + delta.ID}}, nil
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
	return value.OracleEvidence.Valid() && validateOracleClaims(value.OracleClaims) == nil
}

func validateRunEvidence(values []RunIdentity) []string {
	for _, value := range values {
		if !value.StartVerified {
			return []string{"compared run has no verified decision-bound Worker start: " + value.RunID}
		}
		if !value.TerminalAccepted {
			return []string{"compared run has no accepted terminal result: " + value.RunID}
		}
	}
	return nil
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

type derivedDelta struct {
	ID                string
	BaselineFailures  int
	CandidateFailures int
	HeldOut           bool
	Required          bool
}

func derivedDeltas(policy Policy, baseline, candidate []RunIdentity) ([]derivedDelta, error) {
	required := map[string]bool{}
	for _, id := range policy.RequiredClaims {
		required[id] = true
	}
	values := append(append([]RunIdentity(nil), baseline...), candidate...)
	claims := map[string]derivedDelta{}
	expected := map[string]bool{}
	for index, value := range values {
		seen := map[string]bool{}
		for _, claim := range value.OracleClaims {
			seen[claim.ID] = true
			if index == 0 {
				expected[claim.ID] = claim.HeldOut
			} else if heldOut, exists := expected[claim.ID]; !exists || heldOut != claim.HeldOut {
				return nil, errors.New("oracle claim set differs across compared runs")
			}
			delta, exists := claims[claim.ID]
			if !exists {
				delta = derivedDelta{ID: claim.ID, HeldOut: claim.HeldOut, Required: required[claim.ID]}
			}
			if index < len(baseline) {
				if !claim.Passed {
					delta.BaselineFailures++
				}
			} else if !claim.Passed {
				delta.CandidateFailures++
			}
			claims[claim.ID] = delta
		}
		if len(seen) != len(expected) {
			return nil, errors.New("oracle claim set differs across compared runs")
		}
		for id := range expected {
			if !seen[id] {
				return nil, errors.New("oracle claim set differs across compared runs")
			}
		}
	}
	for id := range required {
		if _, exists := claims[id]; !exists {
			return nil, errors.New("required claim is absent from objective oracle evidence: " + id)
		}
	}
	ids := make([]string, 0, len(claims))
	for id := range claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]derivedDelta, 0, len(ids))
	for _, id := range ids {
		result = append(result, claims[id])
	}
	return result, nil
}
