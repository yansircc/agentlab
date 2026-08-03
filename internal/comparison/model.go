package comparison

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

type Verdict string

const (
	SupportedImprovement Verdict = "supported_improvement"
	SupportedRegression  Verdict = "supported_regression"
	Equivalent           Verdict = "equivalent"
	Inconclusive         Verdict = "inconclusive"
	Invalid              Verdict = "invalid"
)

type Policy struct {
	MinimumRepetitions int      `json:"minimum_repetitions"`
	RequiredClaims     []string `json:"required_claims"`
}

type ClaimDelta struct {
	ClaimID           string `json:"claim_id"`
	BaselineFailures  int    `json:"baseline_failures"`
	CandidateFailures int    `json:"candidate_failures"`
	HeldOut           bool   `json:"held_out"`
}

type MetricDelta struct {
	Metric    string `json:"metric"`
	Baseline  string `json:"baseline"`
	Candidate string `json:"candidate"`
}

type ValidityFact struct {
	Kind   string `json:"kind"`
	Valid  bool   `json:"valid"`
	Detail string `json:"detail"`
}

type Observation struct {
	ID            string         `json:"id"`
	CandidateID   string         `json:"candidate_id"`
	BaselineRuns  []string       `json:"baseline_runs"`
	CandidateRuns []string       `json:"candidate_runs"`
	Policy        Policy         `json:"policy"`
	ClaimDeltas   []ClaimDelta   `json:"claim_deltas"`
	MetricDeltas  []MetricDelta  `json:"metric_deltas,omitempty"`
	ValidityFacts []ValidityFact `json:"validity_facts"`
}

type RunIdentity struct {
	RunID           string
	Origin          Origin
	Intervention    bool
	WorkerInput     artifact.Ref
	Harness         artifact.Ref
	Trial           artifact.Ref
	Candidate       artifact.Ref
	Adapter         artifact.Ref
	OracleSet       artifact.Ref
	Fixture         artifact.Ref
	FixtureReset    artifact.Ref
	FixtureBaseline artifact.Ref
	EvidencePolicy  artifact.Ref
	StopPolicy      artifact.Ref
	WorkerRuntime   artifact.Ref
	Environment     artifact.Ref
}

type Origin string

const (
	FreshOrigin  Origin = "fresh"
	SpliceOrigin Origin = "splice"
)

type Result struct {
	Observation Observation `json:"observation"`
	Verdict     Verdict     `json:"verdict"`
	Reasons     []string    `json:"reasons"`
}

func (o Observation) Validate() error {
	if o.ID == "" || o.CandidateID == "" || len(o.BaselineRuns) == 0 || len(o.CandidateRuns) == 0 || o.Policy.MinimumRepetitions < 2 || len(o.Policy.RequiredClaims) == 0 || len(o.ValidityFacts) == 0 {
		return errors.New("comparison identity, runs, repetition policy, claims, and validity facts are required")
	}
	if len(o.BaselineRuns) > 100 || len(o.CandidateRuns) > 100 || len(o.ClaimDeltas) > 100 || len(o.MetricDeltas) > 100 || len(o.ValidityFacts) > 100 {
		return errors.New("comparison collections exceed bounds")
	}
	return validateUnique(o)
}

func validateUnique(o Observation) error {
	seen := map[string]bool{}
	for _, id := range append(append([]string(nil), o.BaselineRuns...), o.CandidateRuns...) {
		if id == "" || seen[id] {
			return errors.New("comparison run ids are absent or duplicated")
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, id := range o.Policy.RequiredClaims {
		if id == "" || seen[id] {
			return errors.New("required claims are absent or duplicated")
		}
		seen[id] = true
	}
	for _, delta := range o.ClaimDeltas {
		if delta.ClaimID == "" || delta.BaselineFailures < 0 || delta.CandidateFailures < 0 {
			return errors.New("claim delta is invalid")
		}
	}
	for _, fact := range o.ValidityFacts {
		if fact.Kind == "" || fact.Detail == "" {
			return errors.New("validity fact is incomplete")
		}
	}
	return nil
}
