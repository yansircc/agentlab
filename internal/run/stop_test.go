package run

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
)

func TestOwnedRunnerAdmitsDurableStopBeforeExit(t *testing.T) {
	root := t.TempDir()
	op, err := Open(root, "test-experiment", "owned-stop")
	if err != nil {
		t.Fatal(err)
	}
	bindTestManifest(t, op)
	t.Setenv("AGENTLAB_HELPER", "1")
	finished := make(chan error, 1)
	go func() {
		_, err := op.Start(context.Background(), "owned-stop", StartSpec{
			PublicCommand:     []string{os.Args[0], "-test.run=TestHelperProcess", "--", "silent"},
			PublicEnvironment: map[string]string{"AGENTLAB_HELPER": "1"},
			Policy: StopPolicy{
				FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second,
				HardIdleTimeout: 4 * time.Second, OwnsWorkerProcess: true,
			},
		})
		finished <- err
	}()
	waitForRunStart(t, op)
	requested, err := op.RequestStop("test_stop")
	if err != nil {
		t.Fatal(err)
	}
	if requested.ID == "" || requested.Admitted {
		t.Fatalf("owned stop request = %#v", requested)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("stopped worker reported terminal success")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owned worker did not observe stop request")
	}
	status, err := op.Status(processidentity.SystemProber{})
	if err != nil {
		t.Fatal(err)
	}
	if !status.StopRequested || status.Health != HealthAbandoned || status.ProcessLiveness != ProcessDead {
		t.Fatalf("stopped status = %#v", status)
	}
	records, err := op.Inspect(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	stopSequence, exitSequence := uint64(0), uint64(0)
	for _, record := range records {
		if record.Kind == eventStopRequested {
			stopSequence = record.Sequence
		}
		if record.Kind == eventProcessExited {
			exitSequence = record.Sequence
		}
	}
	if stopSequence == 0 || exitSequence == 0 || stopSequence >= exitSequence {
		t.Fatalf("stop was not durable before exit: stop=%d exit=%d", stopSequence, exitSequence)
	}
}

func TestAttachedStopNeverSignalsExternalProcess(t *testing.T) {
	worker := exec.Command("sleep", "5")
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = worker.Process.Kill()
		_, _ = worker.Process.Wait()
	}()
	identity, err := processidentity.CaptureProcess(worker.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := Open(t.TempDir(), "test-experiment", "attached-stop")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte(`{"offset":0}`), Policy: policy, Identity: &identity, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	result, err := op.RequestStop("test_stop")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Admitted {
		t.Fatalf("attached stop was not admitted: %#v", result)
	}
	if err := syscall.Kill(worker.Process.Pid, 0); err != nil {
		t.Fatalf("external process was signaled: %v", err)
	}
	status, err := op.Status(processidentity.SystemProber{})
	if err != nil {
		t.Fatal(err)
	}
	if status.Health != HealthAbandoned || status.ProcessLiveness != ProcessAlive {
		t.Fatalf("attached stopped status = %#v", status)
	}
}

func waitForRunStart(t *testing.T, operation *Operation) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		records, err := operation.Inspect(0, 1)
		if err == nil && len(records) == 1 && records[0].Kind == eventProcessStarted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not start")
}
