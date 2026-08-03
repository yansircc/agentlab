package tool

import (
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
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{{Ref: "coder-profile", ExperimentID: "exp", RunID: "coder", Role: effect.CoderStart, SessionPath: t.TempDir() + "/coder.jsonl", Identity: testIdentity(t), Policy: policy, Coder: &coder}})
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

func testIdentity(t *testing.T) piadapter.IdentityConfig {
	t.Helper()
	return piadapter.IdentityConfig{SDKRoot: t.TempDir(), AdapterDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "provider", Model: "model", ThinkingPolicy: "off", CompactionPolicy: "off"}
}
