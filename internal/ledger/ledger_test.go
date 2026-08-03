package ledger

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReplayAndBoundedInspect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	log := Open(path)
	for i := range 3 {
		if _, err := log.Append(time.Unix(int64(i+1), 0), "fact", map[string]int{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	reopened := Open(path)
	records, err := reopened.Read(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Sequence != 2 {
		t.Fatalf("unexpected page: %#v", records)
	}
	if _, err := reopened.Read(0, 0); err == nil {
		t.Fatal("unbounded inspect was accepted")
	}
}

func TestBoundedInspectStillValidatesUnreturnedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	log := Open(path)
	if _, err := log.Append(time.Unix(1, 0), "fact", struct{}{}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("corrupt-tail\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	if records, err := log.Read(0, 1); !errors.Is(err, ErrCorrupt) || len(records) != 1 {
		t.Fatalf("records=%#v err=%v", records, err)
	}
}

func TestLedgerRejectsUnknownFieldsAndOversizedRecords(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.jsonl")
	line := `{"sequence":1,"at":"2026-08-03T00:00:00Z","kind":"fact","data":{},"shadow":true}` + "\n"
	if err := os.WriteFile(unknown, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(unknown).Replay(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("unknown field error = %v", err)
	}
	oversized := filepath.Join(dir, "oversized.jsonl")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte{'x'}, maxRecordBytes+2), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(oversized).Replay(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("oversized record error = %v", err)
	}
}

func TestReplayFailsClosedOnPartialOrCorruptRecord(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "partial.jsonl")
	if err := os.WriteFile(partial, []byte(`{"sequence":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(partial).Replay(); !errors.Is(err, ErrPartialFinal) {
		t.Fatalf("partial record error = %v", err)
	}
	corrupt := filepath.Join(dir, "corrupt.jsonl")
	if err := os.WriteFile(corrupt, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(corrupt).Replay(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("corrupt record error = %v", err)
	}
}
