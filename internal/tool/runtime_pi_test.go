package tool

import (
	"path/filepath"
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
