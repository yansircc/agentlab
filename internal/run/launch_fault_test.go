package run

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/processidentity"
)

func TestSpawnFailureTerminatesAllocatedAttempt(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "spawn-failure")
	bindTestManifest(t, op)
	spec := ownedTestSpec("clean")
	spec.PublicCommand = []string{filepath.Join(t.TempDir(), "missing-worker")}
	if _, err := op.Start(context.Background(), "spawn-failure", spec); err == nil {
		t.Fatal("missing executable launch succeeded")
	}
	attempts := readAttempts(t, op)
	if len(attempts) != 1 || !attempts[0].terminated || attempts[0].identity != nil {
		t.Fatalf("spawn failure state = %#v", attempts)
	}
}

func TestIdentityReceiptFailureKillsAndTerminatesAttempt(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "receipt-failure")
	bindTestManifest(t, op)
	var spawned processidentity.Identity
	op.recordAttemptSpawn = func(_ *launchAttempt, identity processidentity.Identity) error {
		spawned = identity
		return errors.New("injected identity receipt failure")
	}
	t.Setenv("AGENTLAB_HELPER", "1")
	if _, err := op.Start(context.Background(), "receipt-failure", ownedTestSpec("silent")); err == nil {
		t.Fatal("identity receipt failure was ignored")
	}
	if records, err := op.ledger.Replay(); err != nil || len(records) != 0 {
		t.Fatalf("run ledger = %#v, %v", records, err)
	}
	attempts := readAttempts(t, op)
	if len(attempts) != 1 || !attempts[0].terminated || attempts[0].identity != nil {
		t.Fatalf("identity receipt failure state = %#v", attempts)
	}
	if spawned.PID == 0 || (processidentity.SystemProber{}).Observe(spawned) == processidentity.Matches {
		t.Fatalf("spawned process was not proven dead: %#v", spawned)
	}
}

func TestPartialStartAppendKillsAttemptAndLeavesRunFailClosed(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "partial")
	bindTestManifest(t, op)
	realAppend := op.appendRun
	op.appendRun = func(at time.Time, kind string, value any) (ledger.Record, error) {
		if kind != eventProcessStarted {
			return realAppend(at, kind, value)
		}
		if err := os.WriteFile(filepath.Join(op.dir, "events.jsonl"), []byte(`{"sequence":1`), 0o600); err != nil {
			return ledger.Record{}, err
		}
		return ledger.Record{}, errors.New("injected partial append")
	}
	t.Setenv("AGENTLAB_HELPER", "1")
	if _, err := op.Start(context.Background(), "partial", ownedTestSpec("silent")); err == nil {
		t.Fatal("partial append was ignored")
	}
	if _, err := op.ledger.Replay(); !errors.Is(err, ledger.ErrPartialFinal) {
		t.Fatalf("partial ledger error = %v", err)
	}
	attempts := readAttempts(t, op)
	if len(attempts) != 1 || !attempts[0].terminated || attempts[0].identity == nil {
		t.Fatalf("partial append attempt = %#v", attempts)
	}
	op.appendRun = realAppend
	if _, err := op.Start(context.Background(), "partial", ownedTestSpec("silent")); !errors.Is(err, ledger.ErrPartialFinal) {
		t.Fatalf("corrupt run did not fail closed: %v", err)
	}
}

func TestLivePreparedOrphanRequiresIdentityVerifiedTermination(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "orphan")
	manifest := bindTestManifest(t, op)
	spec := ownedTestSpec("clean")
	digest, err := op.bindOwnedRequest(spec, manifest)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := op.allocateLaunchAttempt(digest)
	if err != nil {
		t.Fatal(err)
	}
	identity := processidentity.Identity{PID: 41, PGID: 41, StartToken: "start", CommandHash: "command", Executable: "worker"}
	if err := attempt.recordSpawn(identity); err != nil {
		t.Fatal(err)
	}
	op.attemptProber = fixedProber(processidentity.Matches)
	terminated := false
	op.terminateIdentity = func(got processidentity.Identity) error {
		if got != identity {
			t.Fatalf("terminated identity = %#v", got)
		}
		terminated = true
		return nil
	}
	if err := op.reconcileLaunchAttempts(); err != nil {
		t.Fatal(err)
	}
	if !terminated {
		t.Fatal("live orphan was not terminated")
	}
	state, err := attempt.state()
	if err != nil || !state.terminated {
		t.Fatalf("reconciled attempt = %#v, %v", state, err)
	}
}
