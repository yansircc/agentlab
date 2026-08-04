package tool

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func TestAppendPiRuntimeProfileIsAtomicAndWriteOnce(t *testing.T) {
	first := testPlanWorkerProfile(t, "baseline", "baseline-run")
	data, err := EncodePiRuntimePlan([]PiRuntimeProfile{first})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pi-runtime-plan.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	child := testPlanWorkerProfile(t, "guided", "guided-run")
	if err := AppendPiRuntimeProfile(path, child); err != nil {
		t.Fatal(err)
	}
	host, err := LoadPiRuntimeHost(path)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := host.Profile("guided")
	if err != nil || profile.Ref != child.Ref || profile.RunID != child.RunID || profile.SessionPath != child.SessionPath || profile.Role != child.Role {
		t.Fatalf("appended profile = %#v, %v", profile, err)
	}
	if err := AppendPiRuntimeProfile(path, child); err != nil {
		t.Fatalf("identical profile retry = %v", err)
	}
	if err := AppendPiRuntimeProfile(path, profile); err != nil {
		t.Fatalf("canonical profile retry = %v", err)
	}
	child.RunID = "different-run"
	if err := AppendPiRuntimeProfile(path, child); err == nil {
		t.Fatal("runtime profile ref was rebound")
	}
}

func TestAppendPiRuntimeProfileRejectsInvalidPlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pi-runtime-plan.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendPiRuntimeProfile(path, testPlanWorkerProfile(t, "guided", "guided-run")); err == nil {
		t.Fatal("invalid runtime plan was mutated")
	}
}

func TestPreparedWorkerRuntimeBindsOneExactForkReceipt(t *testing.T) {
	base := testPlanWorkerProfile(t, "baseline", "baseline-run")
	data, err := EncodePiRuntimePlan([]PiRuntimeProfile{base})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pi-runtime-plan.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	template := testPreparedWorkerRuntime(t, "candidate-worker", "candidate-run")
	if err := AppendPiPreparedWorkerRuntime(path, template); err != nil {
		t.Fatal(err)
	}
	host, err := LoadPiRuntimeHost(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := host.PreparedWorker(template.Ref)
	if err != nil || loaded.Forked != nil || loaded.WorkerRuntime != template.WorkerRuntime || loaded.FreshSessionPath != template.FreshSessionPath {
		t.Fatalf("prepared Worker runtime = %#v, %v", loaded, err)
	}
	if err := AppendPiPreparedWorkerRuntime(path, template); err != nil {
		t.Fatalf("identical prepared Worker retry = %v", err)
	}
	forged := template
	forged.RunID = "other-run"
	if err := AppendPiPreparedWorkerRuntime(path, forged); err == nil {
		t.Fatal("prepared Worker ref was rebound")
	}

	forked := testForkedWorkerBinding()
	if err := BindPiForkedWorkerRuntime(path, template.Ref, forked); err != nil {
		t.Fatal(err)
	}
	if err := BindPiForkedWorkerRuntime(path, template.Ref, forked); err != nil {
		t.Fatalf("identical fork binding retry = %v", err)
	}
	host, err = LoadPiRuntimeHost(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = host.PreparedWorker(template.Ref)
	if err != nil || loaded.Forked == nil || *loaded.Forked != forked {
		t.Fatalf("materialized prepared Worker runtime = %#v, %v", loaded, err)
	}
	forgedFork := forked
	forgedFork.ParentRun = "other-parent"
	if err := BindPiForkedWorkerRuntime(path, template.Ref, forgedFork); err == nil {
		t.Fatal("forked Worker receipt was substituted")
	}
}

func testPlanWorkerProfile(t *testing.T, ref, runID string) PiRuntimeProfile {
	t.Helper()
	launch := testWorkerLaunch(t)
	return PiRuntimeProfile{Ref: ref, ExperimentID: "exp", RunID: runID, Role: effect.WorkerStart, SessionPath: filepath.Join(launch.Launch.RuntimeRoot, runID+".jsonl"), Identity: testIdentity(t), Policy: run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}, WorkerLaunch: launch}
}

func testPreparedWorkerRuntime(t *testing.T, ref, runID string) PiPreparedWorkerRuntime {
	t.Helper()
	launch := testWorkerLaunch(t)
	return PiPreparedWorkerRuntime{
		Ref: ref, ExperimentID: "exp", RunID: runID, WorkerRuntime: testRef(),
		FreshSessionPath: filepath.Join(launch.Launch.RuntimeRoot, "session.jsonl"), Identity: testIdentity(t),
		Policy:       run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true},
		WorkerLaunch: *launch,
	}
}

func testForkedWorkerBinding() PiForkedWorkerBinding {
	ref := artifact.Ref{Scope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Algorithm: "sha256", Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Size: 1}
	return PiForkedWorkerBinding{
		ParentRun: "parent-run", ParentRuntimeRef: "parent-runtime", ChildManifest: ref,
		ForkReceipt: effect.Receipt{IntentID: "fork", Kind: effect.Fork, Evidence: ref},
		Forked:      run.SessionForked{Intent: effect.Intent{ID: "fork", RunID: "parent-run", Kind: effect.Fork, Payload: ref}, ExpectedCheckpoint: ref, ParentSession: ref, ChildSession: ref, ObservedPrefix: ref, AdapterIdentity: ref},
	}
}
