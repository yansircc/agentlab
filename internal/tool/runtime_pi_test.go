package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

func TestPiRuntimeHostDerivesStartPayloadFromBoundProfile(t *testing.T) {
	root := t.TempDir()
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	workerLaunch := testWorkerLaunch(t)
	workerPolicy := policy
	workerPolicy.OwnsWorkerProcess = true
	worker, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "worker-profile", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: filepath.Join(workerLaunch.Launch.RuntimeRoot, "worker.jsonl"), Identity: testIdentity(t), Policy: workerPolicy, WorkerLaunch: workerLaunch}})
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{Root: root, ExperimentID: "exp"}
	intent, err := worker.StartIntent(binding, StartRequest{ID: "worker-start", RunID: "worker", RuntimeRef: "worker-profile"})
	if err != nil || intent.Kind != effect.WorkerStart {
		t.Fatalf("worker intent = %#v, %v", intent, err)
	}
	if _, err := worker.Start(binding, intent, "worker-profile"); err == nil {
		t.Fatal("Pi start accepted an unverified runtime identity")
	}
	data, _ := binding.store().Read(intent.Payload)
	var payload run.StartPayload
	if strictjson.Decode(data, &payload) != nil || payload.Coder != nil {
		t.Fatalf("worker payload = %#v", payload)
	}

	store := binding.store()
	put := func(name string) artifact.Ref {
		ref, err := store.Put([]byte(name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	coderPolicy := policy
	coderPolicy.OwnsWorkerProcess = true
	launch := testCoderLaunch(t)
	sourceSnapshot, workspaceReceipt, capabilityProfile := put("source"), put("workspace"), put("capability")
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "coder-profile", ExperimentID: "exp", RunID: "coder", Role: effect.CoderStart, SessionPath: filepath.Join(launch.RuntimeRoot, "coder.jsonl"), Identity: testIdentity(t), Policy: coderPolicy, CoderSourceSnapshot: sourceSnapshot, CoderWorkspaceReceipt: workspaceReceipt, CoderCapabilityProfile: capabilityProfile, CoderWorkspace: t.TempDir(), CoderLaunch: launch}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.StartIntent(binding, StartRequest{ID: "bad", RunID: "coder", RuntimeRef: "coder-profile"}); err == nil {
		t.Fatal("coder start omitted its handoff")
	}
	handoff := put("handoff")
	if _, err := host.StartIntent(binding, StartRequest{ID: "coder-start", RunID: "coder", RuntimeRef: "coder-profile", Handoff: &handoff}); err == nil {
		t.Fatal("coder start accepted a handoff not rendered by its experiment")
	}
	handoff = renderOwnedHandoff(t, binding)
	intent, err = host.StartIntent(binding, StartRequest{ID: "coder-start", RunID: "coder", RuntimeRef: "coder-profile", Handoff: &handoff})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = store.Read(intent.Payload)
	want := run.CoderProfile{Handoff: handoff, SourceSnapshot: sourceSnapshot, CandidateWorkspace: workspaceReceipt, CapabilityProfile: capabilityProfile}
	if strictjson.Decode(data, &payload) != nil || payload.Coder == nil || *payload.Coder != want {
		t.Fatalf("coder payload = %#v", payload)
	}
}

func TestPiRuntimeHostRejectsSharedSessionsAndUnboundCoderWorkspace(t *testing.T) {
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	launch := testCoderLaunch(t)
	shared := filepath.Join(launch.RuntimeRoot, "session.jsonl")
	workerLaunch := testWorkerLaunch(t)
	workerPolicy := policy
	workerPolicy.OwnsWorkerProcess = true
	worker := PiRuntimeProfile{Ref: "worker", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: shared, Identity: testIdentity(t), Policy: workerPolicy, WorkerLaunch: workerLaunch}
	coderPolicy := policy
	coderPolicy.OwnsWorkerProcess = true
	coder := PiRuntimeProfile{Ref: "coder", ExperimentID: "exp", RunID: "coder", Role: effect.CoderStart, SessionPath: shared, Identity: testIdentity(t), Policy: coderPolicy, CoderSourceSnapshot: testRef(), CoderWorkspaceReceipt: testRef(), CoderCapabilityProfile: testRef(), CoderWorkspace: t.TempDir(), CoderLaunch: launch}
	if _, err := NewPiRuntimeHost([]PiRuntimeProfile{worker, coder}); err == nil {
		t.Fatal("shared Worker and Coder session was accepted")
	}
	coder.SessionPath = filepath.Join(launch.RuntimeRoot, "coder.jsonl")
	coder.CoderWorkspace = ""
	if _, err := NewPiRuntimeHost([]PiRuntimeProfile{coder}); err == nil {
		t.Fatal("coder profile omitted workspace capability")
	}
	coder.CoderWorkspace = launch.RuntimeRoot
	if _, err := NewPiRuntimeHost([]PiRuntimeProfile{coder}); err == nil {
		t.Fatal("coder workspace overlapped its runtime root")
	}
}

func TestPiRuntimePlanRoundTripsOnlyValidProfiles(t *testing.T) {
	launch := testWorkerLaunch(t)
	profile := PiRuntimeProfile{Ref: "worker", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: filepath.Join(launch.Launch.RuntimeRoot, "worker.jsonl"), Identity: testIdentity(t), Policy: run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}, WorkerLaunch: launch}
	data, err := EncodePiRuntimePlan([]PiRuntimeProfile{profile})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePiRuntimeHost(data); err != nil {
		t.Fatalf("runtime plan = %s, %v", data, err)
	}
}

func TestPiWorkerProfileHasNoAttachedOrGenericCommandFallback(t *testing.T) {
	launch := testWorkerLaunch(t)
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	profile := PiRuntimeProfile{Ref: "worker", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: filepath.Join(launch.Launch.RuntimeRoot, "worker.jsonl"), Identity: testIdentity(t), Policy: policy, WorkerLaunch: launch}
	if _, err := NewPiRuntimeHost([]PiRuntimeProfile{profile}); err != nil {
		t.Fatal(err)
	}
	launch.Launch.AllowedExecutables = []string{"/bin/sh"}
	if _, err := NewPiRuntimeHost([]PiRuntimeProfile{profile}); err == nil {
		t.Fatal("worker generic executable capability was accepted")
	}
	launch.Launch.AllowedExecutables = nil
	profile.Policy.OwnsWorkerProcess = false
	if _, err := NewPiRuntimeHost([]PiRuntimeProfile{profile}); err == nil {
		t.Fatal("attached-only Worker profile was accepted")
	}
	command := strings.Join(piWorkerCommand(testIdentity(t), "/node", "/runtime/session.jsonl", "/runtime", "/skill/extension.ts", "/runtime/tools.ts", "task"), "\x00")
	if !strings.Contains(command, "/skill/extension.ts") || strings.Count(command, "--extension") != 2 || !strings.Contains(command, "--no-builtin-tools") || !strings.Contains(command, "deployctl_help,deployctl_deploy,deployctl_status,deployctl_receipt") || strings.Contains(command, ",bash") {
		t.Fatalf("Worker command widened authority: %q", command)
	}
}

func TestPiWorkerLaunchRejectsUnknownHostOracle(t *testing.T) {
	launch := testWorkerLaunch(t)
	launch.HostOracle = HostOracleKind("unbounded-provider-command")
	if launch.Validate() == nil {
		t.Fatal("Worker launch accepted an unknown Host oracle")
	}
}

func TestHostWorkerOracleInvokesExactHostOnlyProducer(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "agentlab")
	if err := os.WriteFile(binary, []byte("exact bundled binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	evidence := run.EvidenceRef{ExperimentID: "exp", RunID: "worker", Sequence: 3}
	data, err := json.Marshal(struct {
		OK   bool `json:"ok"`
		Data struct {
			RunID    string          `json:"run_id"`
			Evidence run.EvidenceRef `json:"evidence"`
		} `json:"data"`
	}{
		OK: true,
		Data: struct {
			RunID    string          `json:"run_id"`
			Evidence run.EvidenceRef `json:"evidence"`
		}{RunID: "worker", Evidence: evidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	planPath := filepath.Join(t.TempDir(), "host", "pi-runtime-plan.json")
	oracle := newHostWorkerOracleWithCommand(planPath, func() (string, error) {
		return binary, nil
	}, func(gotBinary, directory string, args []string) ([]byte, error) {
		called = true
		if gotBinary != binary || directory != filepath.Dir(planPath) || !reflect.DeepEqual(args, []string{"acceptance", "worker-oracle", "-host-root", directory, "-run-id", "worker"}) {
			t.Fatalf("Host oracle invocation = %q in %q with %q", gotBinary, directory, args)
		}
		return data, nil
	})
	if err := oracle(HostOracleDeployctl, "worker", executableDigest(binary)); err != nil || !called {
		t.Fatalf("Host oracle = %v, called = %v", err, called)
	}
	called = false
	if err := oracle(HostOracleKind("unbounded"), "worker", executableDigest(binary)); err == nil || called {
		t.Fatalf("Host oracle accepted an unbound producer: %v, called = %v", err, called)
	}
	if err := oracle(HostOracleDeployctl, "worker", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil {
		t.Fatal("Host oracle accepted a different executable identity")
	}
}

func TestPiLaunchRequiresExplicitValidSecretHandles(t *testing.T) {
	launch := *testCoderLaunch(t)
	launch.SecretEnvironmentHandles = map[string]string{"PROVIDER_TOKEN": "not-a-valid-environment-name"}
	if launch.Validate() == nil {
		t.Fatal("Pi launch accepted an invalid secret handle")
	}
	t.Setenv("PROVIDER_TOKEN", "ambient-provider-credential")
	launch.SecretEnvironmentHandles = map[string]string{"PROVIDER_TOKEN": "MISSING_HOST_CREDENTIAL"}
	if _, err := launch.environment(launch.RuntimeRoot); err == nil {
		t.Fatal("Pi launch fell back to the ambient provider credential")
	}
	t.Setenv("HOST_PROVIDER_CREDENTIAL", "host-provided-credential")
	launch.SecretEnvironmentHandles = map[string]string{"PROVIDER_TOKEN": "HOST_PROVIDER_CREDENTIAL"}
	environment, err := launch.environment(launch.RuntimeRoot)
	if err != nil || !strings.Contains(strings.Join(environment, "\x00"), "PROVIDER_TOKEN=host-provided-credential") {
		t.Fatalf("Pi launch credential environment = %q, %v", environment, err)
	}
}

func testRef() artifact.Ref {
	return artifact.Ref{Scope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
}

func testIdentity(t *testing.T) piadapter.IdentityConfig {
	t.Helper()
	root := t.TempDir()
	return piadapter.IdentityConfig{SDKRoot: root, ContextFilterPath: filepath.Join(root, "context-filter.ts"), AdapterDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "provider", Model: "model", ThinkingPolicy: "off", CompactionPolicy: "off"}
}

func testCoderLaunch(t *testing.T) *PiLaunch {
	t.Helper()
	root := t.TempDir()
	return &PiLaunch{NodePath: filepath.Join(root, "node"), RuntimeRoot: filepath.Join(root, "runtime"), ReadOnlyRoots: []string{root}, AllowedExecutables: []string{filepath.Join(root, "shell")}}
}

func testWorkerLaunch(t *testing.T) *PiWorkerLaunch {
	t.Helper()
	root := t.TempDir()
	return &PiWorkerLaunch{Launch: PiLaunch{NodePath: filepath.Join(root, "node"), RuntimeRoot: filepath.Join(root, "runtime"), ReadOnlyRoots: []string{filepath.Join(root, "sdk")}}, FixtureRoot: filepath.Join(root, "fixture"), DeployctlExecutable: filepath.Join(root, "deployctl"), CandidateExecutable: testRef(), WorkerInput: testRef()}
}
