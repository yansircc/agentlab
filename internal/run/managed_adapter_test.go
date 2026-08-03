package run

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/processidentity"
)

func TestManagedAdapterStopDurablyPrecedesProcessTermination(t *testing.T) {
	op, err := Open(t.TempDir(), "managed-experiment", "coder")
	if err != nil {
		t.Fatal(err)
	}
	bindTestManifest(t, op)
	payload, err := EncodeStartPayload(effect.WorkerStart, StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	startRef, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	started, err := op.BeginManagedAttachedEffect(effect.Intent{ID: "managed-start", RunID: "coder", Kind: effect.WorkerStart, Payload: startRef}, ManagedAttachedSpec{
		Adapter: "managed-test", Policy: policy, Capabilities: RequiredAdapterCapabilities(), Command: []string{"/bin/sleep", "30"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: filepath.Dir(op.dir),
		Ready: func() (string, []byte, error) { return "managed-session", []byte("cursor"), nil },
	})
	if err != nil || started.Receipt.IntentID != "managed-start" {
		t.Fatalf("managed start = %#v, %v", started, err)
	}
	stopPayload, err := EncodeStopPayload(StopPayload{Reason: "material failure"})
	if err != nil {
		t.Fatal(err)
	}
	stopRef, err := op.artifacts.Put(stopPayload)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := op.RequestStopEffect(effect.Intent{ID: "managed-stop", RunID: "coder", Kind: effect.Stop, Payload: stopRef})
	if err != nil || !stopped.Stop.Admitted || stopped.Receipt.IntentID != "managed-stop" {
		t.Fatalf("managed stop = %#v, %v", stopped, err)
	}
	status, err := op.Status(processidentity.SystemProber{})
	if err != nil || status.ProcessLiveness != ProcessDead || !status.StopRequested {
		t.Fatalf("managed status = %#v, %v", status, err)
	}
	records, err := op.Inspect(0, 20)
	if err != nil {
		t.Fatal(err)
	}
	stopAt := -1
	for index, record := range records {
		if record.Kind == eventStopRequested {
			stopAt = index
		}
	}
	if len(records) < 3 || records[0].Kind != eventProcessStarted || stopAt < 1 {
		t.Fatalf("managed lifecycle = %#v", records)
	}
}
