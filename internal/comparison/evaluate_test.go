package comparison

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestComparisonRequiresRepetitionAndExactControlledInputs(t *testing.T) {
	candidate := testRef("c")
	manifests := testManifests(candidate)
	observation := testObservation()
	result, err := Evaluate(observation, manifests, candidate)
	if err != nil || result.Verdict != SupportedImprovement {
		t.Fatalf("supported result = %#v, %v", result, err)
	}

	single := observation
	single.ID = "single"
	single.BaselineRuns, single.CandidateRuns = []string{"b1"}, []string{"c1"}
	result, err = Evaluate(single, manifests, candidate)
	if err != nil || result.Verdict != Inconclusive {
		t.Fatalf("single pair = %#v, %v", result, err)
	}

	mismatched := testManifests(candidate)
	changed := mismatched["c2"]
	changed.WorkerInput = testRef("x")
	mismatched["c2"] = changed
	result, err = Evaluate(observation, mismatched, candidate)
	if err != nil || result.Verdict != Invalid || !strings.Contains(strings.Join(result.Reasons, " "), "controlled") {
		t.Fatalf("mismatched inputs = %#v, %v", result, err)
	}
}

func TestComparisonRejectsCandidateDriftAndInvalidEnvironment(t *testing.T) {
	candidate := testRef("c")
	manifests := testManifests(candidate)
	drifted := manifests["c1"]
	drifted.Candidate = testRef("d")
	manifests["c1"] = drifted
	result, err := Evaluate(testObservation(), manifests, candidate)
	if err != nil || result.Verdict != Invalid {
		t.Fatalf("candidate drift = %#v, %v", result, err)
	}

	manifests = testManifests(candidate)
	observation := testObservation()
	observation.ValidityFacts[0].Valid = false
	result, err = Evaluate(observation, manifests, candidate)
	if err != nil || result.Verdict != Invalid {
		t.Fatalf("invalid environment = %#v, %v", result, err)
	}

	manifests = testManifests(candidate)
	for _, id := range []string{"b1", "b2"} {
		value := manifests[id]
		value.Candidate = candidate
		manifests[id] = value
	}
	result, err = Evaluate(testObservation(), manifests, candidate)
	if err != nil || result.Verdict != Invalid {
		t.Fatalf("candidate-as-baseline = %#v, %v", result, err)
	}
}

func TestComparisonRejectsTrialMultisetMismatch(t *testing.T) {
	candidate := testRef("c")
	manifests := testManifests(candidate)
	changed := manifests["c2"]
	changed.Trial = testRef("different-trial")
	manifests["c2"] = changed
	result, err := Evaluate(testObservation(), manifests, candidate)
	if err != nil || result.Verdict != Invalid || !strings.Contains(strings.Join(result.Reasons, " "), "trial") {
		t.Fatalf("trial mismatch = %#v, %v", result, err)
	}
}

func TestEveryControlledInputChangeInvalidatesComparison(t *testing.T) {
	mutations := map[string]func(*RunIdentity){
		"worker input":     func(value *RunIdentity) { value.WorkerInput = testRef("changed-worker") },
		"harness":          func(value *RunIdentity) { value.Harness = testRef("changed-harness") },
		"adapter":          func(value *RunIdentity) { value.Adapter = testRef("changed-adapter") },
		"oracle set":       func(value *RunIdentity) { value.OracleSet = testRef("changed-oracle") },
		"fixture":          func(value *RunIdentity) { value.Fixture = testRef("changed-fixture") },
		"fixture baseline": func(value *RunIdentity) { value.FixtureBaseline = testRef("changed-baseline") },
		"evidence policy":  func(value *RunIdentity) { value.EvidencePolicy = testRef("changed-evidence") },
		"stop policy":      func(value *RunIdentity) { value.StopPolicy = testRef("changed-stop") },
		"worker runtime":   func(value *RunIdentity) { value.WorkerRuntime = testRef("changed-runtime") },
		"environment":      func(value *RunIdentity) { value.Environment = testRef("changed-environment") },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := testRef("c")
			manifests := testManifests(candidate)
			changed := manifests["c2"]
			mutate(&changed)
			manifests["c2"] = changed
			result, err := Evaluate(testObservation(), manifests, candidate)
			if err != nil || result.Verdict != Invalid {
				t.Fatalf("controlled input mutation = %#v, %v", result, err)
			}
		})
	}
}

func TestHeldOutRegressionCannotHideOutsideRequiredClaims(t *testing.T) {
	candidate := testRef("c")
	observation := testObservation()
	observation.ClaimDeltas = append(observation.ClaimDeltas, ClaimDelta{ClaimID: "held-out", BaselineFailures: 0, CandidateFailures: 1, HeldOut: true})
	result, err := Evaluate(observation, testManifests(candidate), candidate)
	if err != nil || result.Verdict != SupportedRegression {
		t.Fatalf("held-out regression = %#v, %v", result, err)
	}
}

func testObservation() Observation {
	return Observation{
		ID: "comparison-1", CandidateID: "candidate-1",
		BaselineRuns: []string{"b1", "b2"}, CandidateRuns: []string{"c1", "c2"},
		Policy:        Policy{MinimumRepetitions: 2, RequiredClaims: []string{"claim-1"}},
		ClaimDeltas:   []ClaimDelta{{ClaimID: "claim-1", BaselineFailures: 2, CandidateFailures: 0, HeldOut: true}},
		ValidityFacts: []ValidityFact{{Kind: "environment", Valid: true, Detail: "equivalent enough"}},
	}
}

func testManifests(candidate artifact.Ref) map[string]RunIdentity {
	stable := RunIdentity{
		WorkerInput: testRef("w"), Harness: testRef("h"), Adapter: testRef("a"), OracleSet: testRef("o"),
		Fixture: testRef("f"), FixtureBaseline: testRef("fb"), EvidencePolicy: testRef("e"), StopPolicy: testRef("s"), WorkerRuntime: testRef("r"), Environment: testRef("v"),
	}
	result := map[string]RunIdentity{}
	for _, value := range []struct{ id, trial string }{{"b1", "1"}, {"b2", "2"}, {"c1", "1"}, {"c2", "2"}} {
		current := stable
		current.RunID, current.Trial, current.FixtureReset = value.id, testRef(value.trial), testRef("reset-"+value.id)
		if value.id[0] == 'c' {
			current.Candidate = candidate
		} else {
			current.Candidate = testRef("b")
		}
		result[value.id] = current
	}
	return result
}

func testRef(character string) artifact.Ref {
	sum := sha256.Sum256([]byte(character))
	return artifact.Ref{Algorithm: "sha256", Digest: hex.EncodeToString(sum[:]), Size: 1}
}
