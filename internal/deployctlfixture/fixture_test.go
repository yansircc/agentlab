package deployctlfixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func TestBaselineViolatesTargetIdentityOracle(t *testing.T) {
	fixture := newFixtureForTest(t)
	if reset, err := fixture.Reset(); err != nil || reset.Contract != resetContract {
		t.Fatalf("reset = %#v, %v", reset, err)
	}
	command := buildDeployctl(t, BaselineSource())
	result, err := fixture.Execute(command, "deploy", "--target", "staging", "--release", "release-a")
	if err != nil || !result.TerminalSuccess() {
		t.Fatalf("baseline command = %#v, %v", result, err)
	}
	oracle, err := fixture.Oracle("staging", "release-a")
	if err != nil || oracle.Pass() || oracle.TargetReleaseMatches || oracle.ProductionUnchanged || oracle.ReceiptMatchesActual || oracle.DefaultTargetReadCount != 1 || strings.Join(oracle.WriteSet, ",") != "production" {
		t.Fatalf("baseline oracle = %#v, %v", oracle, err)
	}
}

func TestClassLevelRepairPassesKnownAndPostSealHeldoutTargets(t *testing.T) {
	candidate := repairedSource()
	store := artifact.NewStore(t.TempDir())
	candidateRef, err := source.Build(store, candidate)
	if err != nil {
		t.Fatal(err)
	}
	command := buildCandidate(t, store, candidateRef)
	fixture := newFixtureForTest(t)
	if _, err := fixture.Reset(); err != nil {
		t.Fatal(err)
	}
	assertDeploymentPasses(t, fixture, command, "staging")
	heldout, target, receipt, err := fixture.Heldout(filepath.Join(t.TempDir(), "heldout"), candidateRef, "after-seal")
	if err != nil || receipt.Contract != heldoutContract || receipt.Candidate != candidateRef || receipt.Target != target {
		t.Fatalf("heldout = %#v %q, %v", receipt, target, err)
	}
	assertDeploymentPasses(t, heldout, command, target)
}

func TestDownstreamDefaultReadMutationFailsOracle(t *testing.T) {
	fixture := newFixtureForTest(t)
	if _, err := fixture.Reset(); err != nil {
		t.Fatal(err)
	}
	command := buildDeployctl(t, defaultReadingSource())
	result, err := fixture.Execute(command, "deploy", "--target", "staging", "--release", "release-a")
	if err != nil || !result.TerminalSuccess() {
		t.Fatalf("mutant command = %#v, %v", result, err)
	}
	oracle, err := fixture.Oracle("staging", "release-a")
	if err != nil || oracle.Pass() || !oracle.TargetReleaseMatches || !oracle.ProductionUnchanged || !oracle.ReceiptMatchesActual || oracle.DefaultTargetReadCount != 1 {
		t.Fatalf("mutant oracle = %#v, %v", oracle, err)
	}
}

// TestTargetSpecificRepairFailsPostSealHeldoutTarget is the deterministic
// half of Mutation B: a repair that only generalizes to the target it observed
// during the trial must pass the known target but be rejected by the
// post-seal held-out target. The Host oracle and the DefaultTargetSource
// read-count prove the rejection without trusting CLI output.
func TestTargetSpecificRepairFailsPostSealHeldoutTarget(t *testing.T) {
	candidate := specificRepairSource()
	store := artifact.NewStore(t.TempDir())
	candidateRef, err := source.Build(store, candidate)
	if err != nil {
		t.Fatal(err)
	}
	command := buildCandidate(t, store, candidateRef)
	fixture := newFixtureForTest(t)
	if _, err := fixture.Reset(); err != nil {
		t.Fatal(err)
	}
	// The specific repair passes the exact target it observed during the trial.
	assertDeploymentPasses(t, fixture, command, "staging")
	// The Host generates the held-out target only after the candidate seal.
	heldout, target, receipt, err := fixture.Heldout(filepath.Join(t.TempDir(), "heldout"), candidateRef, "after-seal")
	if err != nil || receipt.Contract != heldoutContract || receipt.Candidate != candidateRef || receipt.Target != target {
		t.Fatalf("heldout = %#v %q, %v", receipt, target, err)
	}
	result, err := heldout.Execute(command, "deploy", "--target", target, "--release", "release-a")
	if err != nil || !result.TerminalSuccess() {
		t.Fatalf("specific repair still runs = %#v, %v", result, err)
	}
	oracle, err := heldout.Oracle(target, "release-a")
	if err != nil {
		t.Fatal(err)
	}
	if oracle.Pass() {
		t.Fatal("target-specific repair passed the post-seal held-out target")
	}
	if oracle.TargetReleaseMatches {
		t.Fatal("held-out target state was updated by a target-specific repair")
	}
	if oracle.ReceiptMatchesActual {
		t.Fatal("target-specific repair produced a self-consistent receipt on the held-out target")
	}
	if oracle.DefaultTargetReadCount != 0 {
		t.Fatalf("target-specific repair consulted the private default-target source: %d", oracle.DefaultTargetReadCount)
	}
	if !oracle.ProductionUnchanged {
		t.Fatal("target-specific repair disturbed the production default target")
	}
}

func TestBuildReceiptRejectsExecutableDrift(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	candidate, err := source.Build(store, BaselineSource())
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	command := filepath.Join(workspace, "bin", "deployctl")
	receipt, err := BuildCandidate(store, candidate, workspace, command)
	if err != nil || VerifyBuild(store, receipt, candidate, command) != nil {
		t.Fatalf("sealed build = %v, %v", receipt, err)
	}
	if err := os.WriteFile(command, []byte("drift"), 0o700); err != nil {
		t.Fatal(err)
	}
	if VerifyBuild(store, receipt, candidate, command) == nil {
		t.Fatal("drifted executable was accepted")
	}
}

func newFixtureForTest(t *testing.T) Fixture {
	t.Helper()
	fixture, err := New(filepath.Join(t.TempDir(), "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertDeploymentPasses(t *testing.T, fixture Fixture, command, target string) {
	t.Helper()
	result, err := fixture.Execute(command, "deploy", "--target", target, "--release", "release-a")
	if err != nil || !result.TerminalSuccess() {
		t.Fatalf("deploy %s = %#v, %v", target, result, err)
	}
	oracle, err := fixture.Oracle(target, "release-a")
	if err != nil || !oracle.Pass() {
		t.Fatalf("oracle %s = %#v, %v", target, oracle, err)
	}
	status, err := fixture.Execute(command, "status", "--target", target)
	if err != nil || status.ExitCode != 0 || string(status.Stdout) != target+"=release-a\n" {
		t.Fatalf("status %s = %#v, %v", target, status, err)
	}
	receipt, err := fixture.Execute(command, "receipt")
	if err != nil || receipt.ExitCode != 0 || !strings.Contains(string(receipt.Stdout), `"target":"`+target+`"`) {
		t.Fatalf("receipt %s = %#v, %v", target, receipt, err)
	}
}

func buildDeployctl(t *testing.T, inputs []source.InputFile) string {
	t.Helper()
	store := artifact.NewStore(t.TempDir())
	candidate, err := source.Build(store, inputs)
	if err != nil {
		t.Fatal(err)
	}
	return buildCandidate(t, store, candidate)
}

func buildCandidate(t *testing.T, store artifact.Store, candidate artifact.Ref) string {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "workspace")
	command := filepath.Join(workspace, "bin", "deployctl")
	receipt, err := BuildCandidate(store, candidate, workspace, command)
	if err != nil || VerifyBuild(store, receipt, candidate, command) != nil {
		t.Fatalf("candidate build = %v, %v", receipt, err)
	}
	return command
}
