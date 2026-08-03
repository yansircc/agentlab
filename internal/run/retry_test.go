package run

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/transaction"
)

var errInjectedAppend = errors.New("injected process_started append failure")

func ownedTestSpec(mode string) StartSpec {
	return StartSpec{
		PublicCommand:     []string{os.Args[0], "-test.run=TestHelperProcess", "--", mode},
		PublicEnvironment: map[string]string{"AGENTLAB_HELPER": "1"},
		Policy:            StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true},
	}
}

func TestKnownStartAppendFailureTerminatesAttemptAndExactRetry(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "test-experiment", "retry")
	bindTestManifest(t, op)
	realAppend := op.appendRun
	op.appendRun = func(at time.Time, kind string, value any) (ledger.Record, error) {
		if kind == eventProcessStarted {
			return ledger.Record{}, errInjectedAppend
		}
		return realAppend(at, kind, value)
	}
	t.Setenv("AGENTLAB_HELPER", "1")
	if _, err := op.Start(context.Background(), "retry", ownedTestSpec("clean")); !errors.Is(err, errInjectedAppend) {
		t.Fatalf("first launch error = %v", err)
	}
	if records, err := op.ledger.Replay(); err != nil || len(records) != 0 {
		t.Fatalf("ledger after rejected attempt = %#v, %v", records, err)
	}
	attempts := readAttempts(t, op)
	if len(attempts) != 1 || attempts[0].identity == nil || !attempts[0].terminated {
		t.Fatalf("rejected attempt state = %#v", attempts)
	}
	if observation := (processidentity.SystemProber{}).Observe(*attempts[0].identity); observation == processidentity.Matches {
		t.Fatal("rejected attempt process still matches its identity")
	}
	firstAttemptID := onlyAttemptID(t, op)

	op.appendRun = realAppend
	if _, err := op.Start(context.Background(), "retry", ownedTestSpec("clean")); err != nil {
		t.Fatalf("exact retry failed: %v", err)
	}
	records, err := op.ledger.Replay()
	if err != nil || len(records) == 0 || records[0].Sequence != 1 || records[0].Kind != eventProcessStarted {
		t.Fatalf("retry ledger = %#v, %v", records, err)
	}
	state, err := replayRun(records)
	if err != nil || state.started.AttemptID == "" || state.started.AttemptID == firstAttemptID {
		t.Fatalf("retry did not admit a distinct attempt: %#v, %v", state.started, err)
	}
}

func TestDurableStartWinsOverReturnedAppendError(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "ambiguous")
	bindTestManifest(t, op)
	realAppend := op.appendRun
	op.appendRun = func(at time.Time, kind string, value any) (ledger.Record, error) {
		record, err := realAppend(at, kind, value)
		if err == nil && kind == eventProcessStarted {
			return ledger.Record{}, errInjectedAppend
		}
		return record, err
	}
	t.Setenv("AGENTLAB_HELPER", "1")
	if _, err := op.Start(context.Background(), "ambiguous", ownedTestSpec("clean")); err != nil {
		t.Fatalf("durable start was not recovered: %v", err)
	}
	if _, err := op.Start(context.Background(), "ambiguous", ownedTestSpec("clean")); !errors.Is(err, ErrRunStarted) {
		t.Fatalf("second launch error = %v", err)
	}
	if attempts := readAttempts(t, op); len(attempts) != 1 {
		t.Fatalf("attempt count = %d", len(attempts))
	}
}

func TestAllocatedOnlyAttemptFailsClosedBeforeRetrySpawn(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "unknown")
	manifest := bindTestManifest(t, op)
	spec := ownedTestSpec("clean")
	digest, err := op.bindOwnedRequest(spec, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.allocateLaunchAttempt(digest); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTLAB_HELPER", "1")
	if _, err := op.Start(context.Background(), "unknown", spec); !errors.Is(err, ErrAttemptUnresolved) {
		t.Fatalf("allocated-only retry error = %v", err)
	}
	if attempts := readAttempts(t, op); len(attempts) != 1 {
		t.Fatalf("retry spawned a second attempt: %d", len(attempts))
	}
}

func TestConflictingRequestFailsBeforeNewAttempt(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "conflict")
	bindTestManifest(t, op)
	realAppend := op.appendRun
	op.appendRun = func(time.Time, string, any) (ledger.Record, error) { return ledger.Record{}, errInjectedAppend }
	t.Setenv("AGENTLAB_HELPER", "1")
	if _, err := op.Start(context.Background(), "conflict", ownedTestSpec("clean")); err == nil {
		t.Fatal("injected failure was not returned")
	}
	before := len(readAttempts(t, op))
	op.appendRun = realAppend
	if _, err := op.Start(context.Background(), "conflict", ownedTestSpec("duplicate")); !errors.Is(err, transaction.ErrValueExists) {
		t.Fatalf("conflicting request error = %v", err)
	}
	if after := len(readAttempts(t, op)); after != before {
		t.Fatalf("conflicting request created an attempt: before=%d after=%d", before, after)
	}
}

func TestConcurrentStartsCreateOneAcceptedAttempt(t *testing.T) {
	op, _ := Open(t.TempDir(), "test-experiment", "concurrent")
	bindTestManifest(t, op)
	t.Setenv("AGENTLAB_HELPER", "1")
	var wait sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := op.Start(context.Background(), "concurrent", ownedTestSpec("clean"))
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, transaction.ErrLeaseHeld) && !errors.Is(err, ErrRunStarted) {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || len(readAttempts(t, op)) != 1 {
		t.Fatalf("successes=%d attempts=%d", successes, len(readAttempts(t, op)))
	}
}

func TestDurableRunRejectsSecondLaunchBeforeChildCreation(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "test-experiment", "duplicate")
	manifest := bindTestManifest(t, op)
	identity := processidentity.Identity{PID: 42, PGID: 42, StartToken: "A", CommandHash: "hash", Executable: "worker"}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	if _, err := op.ledger.Append(time.Now().UTC(), eventProcessStarted, processStarted{AttemptID: "test-attempt", Manifest: manifest, Process: processHandle{Kind: processOwned, Identity: &identity}, Policy: policy}); err != nil {
		t.Fatal(err)
	}
	_, err := op.Start(context.Background(), "duplicate", StartSpec{PublicCommand: []string{"/does-not-exist"}, Policy: policy})
	if !errors.Is(err, ErrRunStarted) {
		t.Fatalf("second launch error = %v", err)
	}
}
