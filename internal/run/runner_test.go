package run

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
)

func TestHardIdleCleansOwnedProcessGroup(t *testing.T) {
	root := t.TempDir()
	op, err := Open(root, "test-experiment", "group")
	if err != nil {
		t.Fatal(err)
	}
	bindTestManifest(t, op)
	t.Setenv("AGENTLAB_HELPER", "1")
	_, err = op.Start(context.Background(), "group", StartSpec{
		PublicCommand:     []string{os.Args[0], "-test.run=TestHelperProcess", "--", "group"},
		PublicEnvironment: map[string]string{"AGENTLAB_HELPER": "1"},
		Policy: StopPolicy{
			FirstEventTimeout: 500 * time.Millisecond,
			SoftIdleTimeout:   50 * time.Millisecond,
			HardIdleTimeout:   100 * time.Millisecond,
			OwnsWorkerProcess: true,
			KillOnHardIdle:    true,
		},
	})
	if err == nil {
		t.Fatal("forced process group reported success")
	}
	records, err := op.Inspect(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	childPID := 0
	for _, record := range records {
		if record.Kind != eventEvidence {
			continue
		}
		var value evidence
		if err := json.Unmarshal(record.Data, &value); err != nil {
			t.Fatal(err)
		}
		data, err := op.artifacts.Read(value.Raw)
		if err != nil {
			t.Fatal(err)
		}
		if pidText, ok := strings.CutPrefix(string(data), "child_pid="); ok {
			childPID, err = strconv.Atoi(pidText)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if childPID == 0 {
		t.Fatal("child process identity was not captured")
	}
	deadline := time.Now().Add(time.Second)
	for syscall.Kill(childPID, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(childPID, 0); err == nil {
		t.Fatalf("child process %d survived owned group cleanup", childPID)
	}
}

func TestOwnedRunnerTerminalAlgebra(t *testing.T) {
	tests := []struct {
		mode string
		want Health
	}{
		{mode: "clean", want: HealthExitedClean},
		{mode: "duplicate", want: HealthTerminalCorrupt},
		{mode: "missing", want: HealthTerminalCorrupt},
		{mode: "nonzero", want: HealthExitedError},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			root := t.TempDir()
			op, err := Open(root, "test-experiment", test.mode)
			if err != nil {
				t.Fatal(err)
			}
			bindTestManifest(t, op)
			t.Setenv("AGENTLAB_HELPER", "1")
			_, err = op.Start(context.Background(), test.mode, StartSpec{
				PublicCommand:     []string{os.Args[0], "-test.run=TestHelperProcess", "--", test.mode},
				PublicEnvironment: map[string]string{"AGENTLAB_HELPER": "1"},
				Policy: StopPolicy{
					FirstEventTimeout: 10 * time.Millisecond,
					SoftIdleTimeout:   100 * time.Millisecond,
					HardIdleTimeout:   200 * time.Millisecond,
					OwnsWorkerProcess: true,
				},
			})
			if test.want == HealthExitedClean && err != nil {
				t.Fatalf("clean terminal rejected: %v", err)
			}
			if test.want != HealthExitedClean && err == nil {
				t.Fatal("invalid terminal reported success")
			}
			status, err := op.Status(fixedProber(processidentity.Dead))
			if err != nil {
				t.Fatal(err)
			}
			if status.Health != test.want {
				t.Fatalf("health = %s, want %s", status.Health, test.want)
			}
		})
	}
}

func TestSilentWorkerCreatesDurableDeadlines(t *testing.T) {
	root := t.TempDir()
	op, err := Open(root, "test-experiment", "silent")
	if err != nil {
		t.Fatal(err)
	}
	bindTestManifest(t, op)
	t.Setenv("AGENTLAB_HELPER", "1")
	_, err = op.Start(context.Background(), "silent", StartSpec{
		PublicCommand:     []string{os.Args[0], "-test.run=TestHelperProcess", "--", "silent"},
		PublicEnvironment: map[string]string{"AGENTLAB_HELPER": "1"},
		Policy: StopPolicy{
			FirstEventTimeout: 20 * time.Millisecond,
			SoftIdleTimeout:   40 * time.Millisecond,
			HardIdleTimeout:   80 * time.Millisecond,
			OwnsWorkerProcess: true,
			KillOnHardIdle:    true,
		},
	})
	if err == nil {
		t.Fatal("terminated silent worker reported success")
	}
	records, err := op.Inspect(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, record := range records {
		found[record.Kind] = true
	}
	for _, kind := range []string{eventFirstTimeout, eventSoftIdle, eventHardIdle, eventProcessExited} {
		if !found[kind] {
			t.Fatalf("missing durable %s fact", kind)
		}
	}
}
