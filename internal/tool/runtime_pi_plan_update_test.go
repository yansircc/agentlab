package tool

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func testPlanWorkerProfile(t *testing.T, ref, runID string) PiRuntimeProfile {
	t.Helper()
	launch := testWorkerLaunch(t)
	return PiRuntimeProfile{Ref: ref, ExperimentID: "exp", RunID: runID, Role: effect.WorkerStart, SessionPath: filepath.Join(launch.Launch.RuntimeRoot, runID+".jsonl"), Identity: testIdentity(t), Policy: run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}, WorkerLaunch: launch}
}
