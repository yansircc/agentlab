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
	result, err := Evaluate(testObservation(), manifests, candidate)
	if err != nil || result.Verdict != SupportedImprovement {
		t.Fatalf("supported result = %#v, %v", result, err)
	}

	single := testObservation()
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
	result, err = Evaluate(testObservation(), mismatched, candidate)
	if err != nil || result.Verdict != Invalid || !strings.Contains(strings.Join(result.Reasons, " "), "controlled") {
		t.Fatalf("mismatched inputs = %#v, %v", result, err)
	}
}

func TestComparisonRequiresVerifiedStartsAndAcceptedTerminals(t *testing.T) {
	candidate := testRef("c")
	for name, mutate := range map[string]func(*RunIdentity){
		"unverified start":  func(value *RunIdentity) { value.StartVerified = false },
		"rejected terminal": func(value *RunIdentity) { value.TerminalAccepted = false },
	} {
		t.Run(name, func(t *testing.T) {
			manifests := testManifests(candidate)
			value := manifests["c1"]
			mutate(&value)
			manifests["c1"] = value
			result, err := Evaluate(testObservation(), manifests, candidate)
			if err != nil || result.Verdict != Invalid || !strings.Contains(strings.Join(result.Reasons, " "), "compared run") {
				t.Fatalf("runtime proof = %#v, %v", result, err)
			}
		})
	}
}

func TestComparisonRejectsCandidateDriftAndMissingObjectiveClaims(t *testing.T) {
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
	value := manifests["c1"]
	value.OracleClaims = nil
	manifests["c1"] = value
	if _, err := Evaluate(testObservation(), manifests, candidate); err == nil {
		t.Fatal("comparison accepted a run without objective oracle claims")
	}

	manifests = testManifests(candidate)
	missingClaim := testObservation()
	missingClaim.Policy.RequiredClaims = []string{"claim-absent-from-oracle"}
	if _, err := Evaluate(missingClaim, manifests, candidate); err == nil || !strings.Contains(err.Error(), "required claim") {
		t.Fatalf("claim/evidence mismatch error = %v", err)
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

func TestComparisonAllowsHostBoundRuntimeForEachIsolatedRun(t *testing.T) {
	candidate := testRef("c")
	manifests := testManifests(candidate)
	for _, id := range []string{"b1", "b2", "c1", "c2"} {
		value := manifests[id]
		value.WorkerRuntime = testRef("runtime-" + id)
		manifests[id] = value
	}
	result, err := Evaluate(testObservation(), manifests, candidate)
	if err != nil || result.Verdict != SupportedImprovement {
		t.Fatalf("candidate-bound runtimes = %#v, %v", result, err)
	}
}

func TestHeldOutRegressionCannotHideOutsideRequiredClaims(t *testing.T) {
	candidate := testRef("c")
	manifests := testManifests(candidate)
	for _, id := range []string{"b1", "b2", "c1", "c2"} {
		value := manifests[id]
		value.OracleClaims = append(value.OracleClaims, OracleClaim{ID: "held-out", Passed: id[0] == 'b', HeldOut: true})
		manifests[id] = value
	}
	result, err := Evaluate(testObservation(), manifests, candidate)
	if err != nil || result.Verdict != SupportedRegression {
		t.Fatalf("held-out regression = %#v, %v", result, err)
	}
}

func TestComparisonRejectsOracleClaimSetDrift(t *testing.T) {
	candidate := testRef("c")
	manifests := testManifests(candidate)
	value := manifests["c2"]
	value.OracleClaims[0].HeldOut = false
	manifests["c2"] = value
	if _, err := Evaluate(testObservation(), manifests, candidate); err == nil || !strings.Contains(err.Error(), "oracle claim set") {
		t.Fatalf("oracle policy drift error = %v", err)
	}
}

func testObservation() Observation {
	return Observation{
		ID: "comparison-1", CandidateID: "candidate-1",
		BaselineRuns: []string{"b1", "b2"}, CandidateRuns: []string{"c1", "c2"},
		Policy: Policy{MinimumRepetitions: 2, RequiredClaims: []string{"claim-1"}},
	}
}

func testManifests(candidate artifact.Ref) map[string]RunIdentity {
	stable := RunIdentity{
		Origin:      FreshOrigin,
		WorkerInput: testRef("w"), Harness: testRef("h"), Adapter: testRef("a"), OracleSet: testRef("o"),
		Fixture: testRef("f"), FixtureBaseline: testRef("fb"), EvidencePolicy: testRef("e"), StopPolicy: testRef("s"), WorkerRuntime: testRef("r"), Environment: testRef("v"),
		StartVerified: true, TerminalAccepted: true,
	}
	result := map[string]RunIdentity{}
	for _, value := range []struct{ id, trial string }{{"b1", "1"}, {"b2", "2"}, {"c1", "1"}, {"c2", "2"}} {
		current := stable
		current.RunID, current.Trial, current.FixtureReset, current.OracleEvidence = value.id, testRef(value.trial), testRef("reset-"+value.id), testRef("oracle-evidence-"+value.id)
		current.OracleClaims = []OracleClaim{{ID: "claim-1", Passed: value.id[0] == 'c', HeldOut: true}}
		if value.id[0] == 'c' {
			current.Candidate = candidate
		} else {
			current.Candidate = testRef("b")
		}
		result[value.id] = current
	}
	return result
}

func TestComparisonRejectsGuidedOrIntervenedRun(t *testing.T) {
	candidate := testRef("c")
	for name, mutate := range map[string]func(*RunIdentity){
		"splice":       func(value *RunIdentity) { value.Origin = SpliceOrigin },
		"intervention": func(value *RunIdentity) { value.Intervention = true },
	} {
		t.Run(name, func(t *testing.T) {
			manifests := testManifests(candidate)
			value := manifests["c1"]
			mutate(&value)
			manifests["c1"] = value
			result, err := Evaluate(testObservation(), manifests, candidate)
			if err != nil || result.Verdict != Invalid || !strings.Contains(strings.Join(result.Reasons, " "), "guided") {
				t.Fatalf("guided identity = %#v, %v", result, err)
			}
		})
	}
}

func testRef(character string) artifact.Ref {
	sum := sha256.Sum256([]byte(character))
	scope := sha256.Sum256([]byte("scope"))
	return artifact.Ref{Scope: hex.EncodeToString(scope[:]), Algorithm: "sha256", Digest: hex.EncodeToString(sum[:]), Size: 1}
}
