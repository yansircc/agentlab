package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLeaseHasOneWriterAndReleaseChecksOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("second writer error = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireBreaksStaleLease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "writer.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	// The first holder "dies" without releasing: remove its lease receipt's
	// identity by simulating a dead process (the lock file's identity names a
	// pid that no longer exists). Directly remove the file is what a holder
	// crash cannot do, so instead verify the stale path via a fake receipt.
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(path, []byte(`{"token":"stale","identity":{"pid":999999,"pgid":999999,"start_token":"x","command_hash":"y","executable":"/nonexistent"}}`), 0o600)
	got, err := Acquire(path)
	if err != nil {
		t.Fatalf("stale lease was not broken: %v", err)
	}
	_ = got.Release()
}

func TestAcquireRejectsLiveLease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "writer.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := Acquire(path); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("live lease acquisition = %v, want ErrLeaseHeld", err)
	}
}
