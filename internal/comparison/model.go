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

type Observation struct {
	ID            string   `json:"id"`
	CandidateID   string   `json:"candidate_id"`
	BaselineRuns  []string `json:"baseline_runs"`
	CandidateRuns []string `json:"candidate_runs"`
	Policy        Policy   `json:"policy"`
}

// OracleEvidence is Host/adapter-produced objective evidence for one terminal
// Worker run. It binds every claim result to the exact candidate, trial, and
// oracle set selected by the Host-owned manifest. It is intentionally not a
// provider-authored comparison field.
type OracleEvidence struct {
	Contract  string        `json:"contract"`
	RunID     string        `json:"run_id"`
	Candidate artifact.Ref  `json:"candidate"`
	Trial     artifact.Ref  `json:"trial"`
	OracleSet artifact.Ref  `json:"oracle_set"`
	Claims    []OracleClaim `json:"claims"`
}

const OracleEvidenceContract = "agentlab.oracle-evidence.v1"

type OracleClaim struct {
	ID      string `json:"id"`
	Passed  bool   `json:"passed"`
	HeldOut bool   `json:"held_out"`
}

type RunIdentity struct {
	RunID            string
	Origin           Origin
	Intervention     bool
	WorkerInput      artifact.Ref
	Harness          artifact.Ref
	Trial            artifact.Ref
	Candidate        artifact.Ref
	Adapter          artifact.Ref
	OracleSet        artifact.Ref
	Fixture          artifact.Ref
	FixtureReset     artifact.Ref
	FixtureBaseline  artifact.Ref
	EvidencePolicy   artifact.Ref
	StopPolicy       artifact.Ref
	WorkerRuntime    artifact.Ref
	Environment      artifact.Ref
	StartVerified    bool
	TerminalAccepted bool
	OracleEvidence   artifact.Ref
	OracleClaims     []OracleClaim
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
	if o.ID == "" || o.CandidateID == "" || len(o.BaselineRuns) == 0 || len(o.CandidateRuns) == 0 || o.Policy.MinimumRepetitions < 2 || len(o.Policy.RequiredClaims) == 0 {
		return errors.New("comparison identity, runs, repetition policy, and claims are required")
	}
	if len(o.BaselineRuns) > 100 || len(o.CandidateRuns) > 100 {
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
	return nil
}

func (value OracleEvidence) Validate() error {
	if value.Contract != OracleEvidenceContract || value.RunID == "" || !value.Candidate.Valid() || !value.Trial.Valid() || !value.OracleSet.Valid() || len(value.Claims) == 0 || len(value.Claims) > 100 {
		return errors.New("oracle evidence is invalid")
	}
	return validateOracleClaims(value.Claims)
}

func validateOracleClaims(values []OracleClaim) error {
	seen := map[string]bool{}
	for _, value := range values {
		if value.ID == "" || seen[value.ID] {
			return errors.New("oracle claim is invalid")
		}
		seen[value.ID] = true
	}
	return nil
}
