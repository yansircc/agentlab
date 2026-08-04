package run

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
)

func TestContinuousWorkerProjectsActiveBeforeTerminal(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "test-experiment", "continuous")
	bindTestManifest(t, op)
	t.Setenv("AGENTLAB_HELPER", "1")
	done := make(chan error, 1)
	go func() {
		_, err := op.Start(context.Background(), "continuous", StartSpec{
			PublicCommand:     []string{os.Args[0], "-test.run=TestHelperProcess", "--", "continuous"},
			PublicEnvironment: map[string]string{"AGENTLAB_HELPER": "1"},
			Policy: StopPolicy{
				FirstEventTimeout: time.Second, SoftIdleTimeout: 100 * time.Millisecond,
				HardIdleTimeout: time.Second, OwnsWorkerProcess: true,
			},
		})
		done <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	observed := false
	var last Status
	var lastErr error
	for time.Now().Before(deadline) {
		reopened, _ := Open(root, "test-experiment", "continuous")
		status, err := reopened.Status(fixedProber(processidentity.Matches))
		last, lastErr = status, err
		if err == nil && status.Health == HealthAliveActive && status.StreamActivity == RecentEvent {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatalf("continuous public events never projected an active live worker: status=%#v err=%v", last, lastErr)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReplayConvergesWithoutFileNotification(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "test-experiment", "notification-loss")
	manifest := bindTestManifest(t, op)
	identity := processidentity.Identity{PID: 42, PGID: 42, StartToken: "start", CommandHash: "command", Executable: "worker"}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	if _, err := op.ledger.Append(time.Unix(1, 0), eventProcessStarted, processStarted{AttemptID: "attempt", Manifest: manifest, Process: processHandle{Kind: processOwned, Identity: &identity}, Policy: policy}); err != nil {
		t.Fatal(err)
	}

	// No wakeup or notification is delivered. A fresh reader converges from the ledger.
	ref, _ := op.artifacts.Put([]byte("public event"))
	if _, err := op.ledger.Append(time.Unix(2, 0), eventEvidence, evidence{Stream: "stdout", Raw: ref, Label: "public_output"}); err != nil {
		t.Fatal(err)
	}
	reopened, _ := Open(root, "test-experiment", "notification-loss")
	status, err := reopened.StatusAt(fixedProber(processidentity.Matches), time.Unix(2, int64(10*time.Millisecond)))
	if err != nil || status.Health != HealthAliveActive || status.EventCount != 2 {
		t.Fatalf("replayed status = %#v, %v", status, err)
	}
}
