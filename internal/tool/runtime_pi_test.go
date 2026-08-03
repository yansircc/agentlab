package tool

import (
	"path/filepath"
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
	worker, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "worker-profile", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: t.TempDir() + "/worker.jsonl", Identity: testIdentity(t), Policy: policy}})
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
	coder := run.CoderProfile{Handoff: put("handoff"), SourceSnapshot: put("source"), CandidateWorkspace: put("workspace"), CapabilityProfile: put("capability")}
	coderPolicy := policy
	coderPolicy.OwnsWorkerProcess = true
	launch := testCoderLaunch(t)
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "coder-profile", ExperimentID: "exp", RunID: "coder", Role: effect.CoderStart, SessionPath: filepath.Join(launch.RuntimeRoot, "coder.jsonl"), Identity: testIdentity(t), Policy: coderPolicy, Coder: &coder, CoderWorkspace: t.TempDir(), CoderLaunch: launch}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.StartIntent(binding, StartRequest{ID: "bad", RunID: "coder", RuntimeRef: "coder-profile"}); err == nil {
		t.Fatal("coder start omitted its handoff")
	}
	wrong := put("wrong")
	if _, err := host.StartIntent(binding, StartRequest{ID: "bad", RunID: "coder", RuntimeRef: "coder-profile", Handoff: &wrong}); err == nil {
		t.Fatal("coder start changed its Host handoff")
	}
	intent, err = host.StartIntent(binding, StartRequest{ID: "coder-start", RunID: "coder", RuntimeRef: "coder-profile", Handoff: &coder.Handoff})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = store.Read(intent.Payload)
	if strictjson.Decode(data, &payload) != nil || payload.Coder == nil || *payload.Coder != coder {
		t.Fatalf("coder payload = %#v", payload)
	}
}

func TestPiRuntimeHostRejectsSharedSessionsAndUnboundCoderWorkspace(t *testing.T) {
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	launch := testCoderLaunch(t)
	shared := filepath.Join(launch.RuntimeRoot, "session.jsonl")
	worker := PiRuntimeProfile{Ref: "worker", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: shared, Identity: testIdentity(t), Policy: policy}
	coderPolicy := policy
	coderPolicy.OwnsWorkerProcess = true
	coder := PiRuntimeProfile{Ref: "coder", ExperimentID: "exp", RunID: "coder", Role: effect.CoderStart, SessionPath: shared, Identity: testIdentity(t), Policy: coderPolicy, Coder: &run.CoderProfile{Handoff: testRef(), SourceSnapshot: testRef(), CandidateWorkspace: testRef(), CapabilityProfile: testRef()}, CoderWorkspace: t.TempDir(), CoderLaunch: launch}
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

func testRef() artifact.Ref {
	return artifact.Ref{Scope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
}

func testIdentity(t *testing.T) piadapter.IdentityConfig {
	t.Helper()
	root := t.TempDir()
	return piadapter.IdentityConfig{SDKRoot: root, ContextFilterPath: filepath.Join(root, "context-filter.ts"), AdapterDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "provider", Model: "model", ThinkingPolicy: "off", CompactionPolicy: "off"}
}

func testCoderLaunch(t *testing.T) *PiCoderLaunch {
	t.Helper()
	root := t.TempDir()
	return &PiCoderLaunch{NodePath: filepath.Join(root, "node"), RuntimeRoot: filepath.Join(root, "runtime"), ReadOnlyRoots: []string{root}, AllowedExecutables: []string{filepath.Join(root, "shell")}}
}
