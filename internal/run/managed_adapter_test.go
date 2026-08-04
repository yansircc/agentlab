package run

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/source"
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
		Ready:    func() (string, []byte, error) { return "managed-session", []byte("cursor"), nil },
		Finalize: func(int) error { return nil },
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
	records := awaitManagedTerminal(t, op)
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

func TestManagedAdapterWaitsRecordsExitAndSettlesTerminal(t *testing.T) {
	op, _ := Open(t.TempDir(), "managed-experiment", "managed-success")
	bindTestManifest(t, op)
	payload, err := EncodeStartPayload(effect.WorkerStart, StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	start, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	finalized := make(chan int, 1)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	_, err = op.BeginManagedAttachedEffect(effect.Intent{ID: "managed-start", RunID: "managed-success", Kind: effect.WorkerStart, Payload: start}, ManagedAttachedSpec{
		Adapter: "managed-test", Policy: policy, Capabilities: RequiredAdapterCapabilities(), Command: []string{"/bin/sh", "-c", "sleep 0.05"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: filepath.Dir(op.dir),
		Ready: func() (string, []byte, error) { return "managed-session", []byte("cursor"), nil }, Finalize: func(code int) error { finalized <- code; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-finalized:
		if code != 0 {
			t.Fatalf("managed exit code = %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("managed finalizer did not receive process exit")
	}
	records := awaitManagedTerminal(t, op)
	if records[len(records)-2].Kind != eventProcessExited || records[len(records)-1].Kind != eventTerminalAccepted {
		t.Fatalf("managed terminal facts = %#v", records)
	}
	status, err := op.Status(processidentity.SystemProber{})
	if err != nil || status.Health != HealthExitedClean || status.ProcessLiveness != ProcessDead {
		t.Fatalf("managed terminal status = %#v, %v", status, err)
	}
}

func TestCoderCompletionSealsOnlyStartedCoderWorkspace(t *testing.T) {
	op, _ := Open(t.TempDir(), "managed-experiment", "coder-completion")
	manifest := bindTestManifest(t, op)
	put := func(value string) artifact.Ref {
		ref, err := op.artifacts.Put([]byte(value))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	original, err := source.Build(op.artifacts, []source.InputFile{{Path: "main.go", Content: []byte("package main\n")}})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := source.Build(op.artifacts, []source.InputFile{{Path: "main.go", Content: []byte("package main\nfunc main() {}\n")}})
	if err != nil {
		t.Fatal(err)
	}
	profile := CoderProfile{Handoff: put("handoff"), SourceSnapshot: original, CandidateWorkspace: put("workspace"), CapabilityProfile: put("capability")}
	cursor := put("cursor")
	identity := processidentity.Identity{PID: 1, PGID: 1, StartToken: "start", CommandHash: "command", Executable: "/bin/sh"}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	if _, err := op.ledger.Append(time.Now().UTC(), eventProcessStarted, processStarted{AttemptID: "attempt", Manifest: manifest, Process: processHandle{Kind: processManaged, Identity: &identity}, Policy: policy, Adapter: &adapterBinding{Adapter: "pi", StreamID: "coder-session", Cursor: cursor, Capabilities: RequiredAdapterCapabilities()}, Coder: &profile}); err != nil {
		t.Fatal(err)
	}
	receipt, err := op.RecordCoderCompletion(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.finishManaged(0, nil); err != nil {
		t.Fatal(err)
	}
	gotReceipt, completion, err := op.CoderCompletionReceipt()
	if err != nil || gotReceipt != receipt || completion.Profile != profile || completion.SessionID != "coder-session" || completion.Candidate != candidate {
		t.Fatalf("coder completion = %v %#v %#v", err, gotReceipt, completion)
	}
	status, err := op.Status(processidentity.SystemProber{})
	if err != nil || status.Health != HealthExitedClean {
		t.Fatalf("completed coder status = %#v, %v", status, err)
	}
}

func awaitManagedTerminal(t *testing.T, op *Operation) []ledger.Record {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		records, err := op.Inspect(0, 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) >= 2 && (records[len(records)-1].Kind == eventTerminalAccepted || records[len(records)-1].Kind == eventTerminalRejected) {
			return records
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("managed process did not write terminal facts")
	return nil
}
