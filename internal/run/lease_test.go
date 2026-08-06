package run

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/transaction"
)

// TestAcquireProducerLeaseWaitsForLiveWriter pins the terminal-fact race: a
// managed process can exit while the same Host still holds the producer lease
// for an adapter evidence commit. The terminal and completion recorders must
// wait for that live writer and then succeed, not fail closed.
func TestAcquireProducerLeaseWaitsForLiveWriter(t *testing.T) {
	root := t.TempDir()
	held, err := transaction.Acquire(filepath.Join(root, "producer.lock"))
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		lease *transaction.Lease
		err   error
	}
	done := make(chan result, 1)
	go func() {
		lease, acquireErr := acquireProducerLease(root)
		done <- result{lease: lease, err: acquireErr}
	}()
	time.Sleep(100 * time.Millisecond)
	select {
	case result := <-done:
		t.Fatalf("acquireProducerLease did not wait for the live writer: %v, %v", result.lease, result.err)
	default:
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.lease == nil {
			t.Fatalf("acquireProducerLease after release = %v, %v", result.lease, result.err)
		}
		if err := result.lease.Release(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("acquireProducerLease did not acquire after the writer released")
	}
}
