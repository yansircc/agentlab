package preparation

import (
	"encoding/json"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestRenderWorkerInputReadsExactlySealedPublicRefs(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	intent, err := store.Put([]byte("deploy release-a to staging"))
	if err != nil {
		t.Fatal(err)
	}
	public, err := store.Put([]byte("deployctl is the public interface"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(WorkerInput{Contract: workerInputContract, UserIntentRef: intent, PublicArtifacts: []artifact.Ref{public}})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := store.Put(encoded)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := RenderWorkerInput(store, sealed)
	if err != nil || prompt != "deploy release-a to staging\n\ndeployctl is the public interface" {
		t.Fatalf("rendered prompt = %q, %v", prompt, err)
	}
	invalid, err := store.Put([]byte(`{"contract":"agentlab.worker-input.v1","user_intent_ref":` + string(mustJSON(t, intent)) + `,"public_artifacts":[` + string(mustJSON(t, intent)) + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderWorkerInput(store, invalid); err == nil {
		t.Fatal("duplicate sealed ref was accepted")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
