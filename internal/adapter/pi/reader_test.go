package pi

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memorySink struct{ batches []Batch }

func (s *memorySink) Commit(_ Cursor, batch Batch) error {
	s.batches = append(s.batches, batch)
	return nil
}

func TestReaderBoundsNewRecordsButCanDiscardPreAttachPartial(t *testing.T) {
	t.Run("new oversized record", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "session.jsonl")
		if err := os.WriteFile(path, []byte(`{"type":"session","version":3,"id":"session-1"}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cursor, err := Attach(path)
		if err != nil {
			t.Fatal(err)
		}
		appendBytes(t, path, bytes.Repeat([]byte{'x'}, maxLineBytes+2))
		if _, err := ReadNew(path, cursor, &memorySink{}); !errors.Is(err, ErrLineTooLarge) {
			t.Fatalf("oversized record error = %v", err)
		}
	})
	t.Run("pre-attach partial is discarded in bounded chunks", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "session.jsonl")
		data := append([]byte(`{"type":"session","version":3,"id":"session-1"}`+"\n"), bytes.Repeat([]byte{'x'}, maxLineBytes+2)...)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		cursor, err := Attach(path)
		if err != nil {
			t.Fatal(err)
		}
		appendBytes(t, path, []byte("\n"+`{"type":"message","id":"new","message":{"role":"user","content":"hello"}}`+"\n"))
		sink := &memorySink{}
		if _, err := ReadNew(path, cursor, sink); err != nil {
			t.Fatal(err)
		}
		if len(sink.batches) != 2 || len(sink.batches[0].Events) != 0 || len(sink.batches[1].Events) != 1 {
			t.Fatalf("discard batches = %#v", sink.batches)
		}
	})
}

type failingSink struct{}

func (failingSink) Commit(Cursor, Batch) error { return errors.New("commit failed") }

func TestIncrementalReaderExcludesThinkingAndResumesAtDurableCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	initial := strings.Join([]string{
		`{"type":"session","version":3,"id":"session-1","timestamp":"2026-08-03T00:00:00Z","cwd":"/tmp"}`,
		`{"type":"message","id":"old","parentId":null,"timestamp":"2026-08-03T00:00:01Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"old-private"},{"type":"text","text":"old-public"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	cursor, err := Attach(path)
	if err != nil {
		t.Fatal(err)
	}
	appended := `{"type":"message","id":"new","parentId":"old","timestamp":"2026-08-03T00:00:02Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"new-private-secret"},{"type":"text","text":"public answer"},{"type":"toolCall","id":"call-1","name":"read","arguments":{"path":"README.md"}}]}}` + "\n"
	appendBytes(t, path, []byte(appended))
	sink := &memorySink{}
	cursor, err = ReadNew(path, cursor, sink)
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 1 || len(sink.batches[0].Events) != 2 || len(sink.batches[0].Exclusions) != 1 {
		t.Fatalf("unexpected translated batch: %#v", sink.batches)
	}
	for _, event := range sink.batches[0].Events {
		if strings.Contains(string(event.Raw), "new-private-secret") || strings.Contains(event.CompactText, "new-private-secret") {
			t.Fatal("private thinking crossed persistence boundary")
		}
	}
	if sink.batches[0].Events[1].CorrelationID != "call-1" {
		t.Fatalf("tool correlation lost: %#v", sink.batches[0].Events[1])
	}

	partial := `{"type":"message","id":"result","parentId":"new","timestamp":"2026-08-03T00:00:03Z","message":{"role":"toolResult","toolCallId":"call-1","toolName":"read","content":[{"type":"text","text":"ok"}],"isError":false}}`
	appendBytes(t, path, []byte(partial))
	beforePartial := cursor
	cursor, err = ReadNew(path, cursor, sink)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != beforePartial || len(sink.batches) != 1 {
		t.Fatal("partial final record advanced durable cursor")
	}
	appendBytes(t, path, []byte("\n"))
	cursor, err = ReadNew(path, cursor, sink)
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 2 || sink.batches[1].Events[0].Kind != "tool_result" || sink.batches[1].Events[0].CorrelationID != "call-1" {
		t.Fatalf("tool result was not recovered: %#v", sink.batches)
	}
	if _, err := ReadNew(path, cursor, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 2 {
		t.Fatal("restart duplicated admitted records")
	}
}

func TestCommitFailureDoesNotAdvanceCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	initial := `{"type":"session","version":3,"id":"session-1"}` + "\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	cursor, err := Attach(path)
	if err != nil {
		t.Fatal(err)
	}
	appendBytes(t, path, []byte(`{"type":"message","id":"new","message":{"role":"user","content":"hello"}}`+"\n"))
	next, err := ReadNew(path, cursor, failingSink{})
	if err == nil {
		t.Fatal("sink failure was ignored")
	}
	if next != cursor {
		t.Fatalf("cursor advanced on failed commit: before=%#v after=%#v", cursor, next)
	}
}

func TestPiStopReasonProducesObservationWithoutCommittingRunTerminalState(t *testing.T) {
	line := []byte(`{"type":"message","id":"final","message":{"role":"assistant","content":[{"type":"text","text":"done"}],"stopReason":"stop"}}`)
	batch, err := translate(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 2 || batch.Events[0].Kind != "assistant_message" || batch.Events[1].Kind != "terminal" || batch.Events[1].Label != "pi_turn_completed" {
		t.Fatalf("Pi terminal observation = %#v", batch)
	}
	toolUse := []byte(`{"type":"message","id":"tool","message":{"role":"assistant","content":[],"stopReason":"toolUse"}}`)
	batch, err = translate(toolUse)
	if err != nil || len(batch.Events) != 0 {
		t.Fatalf("tool-use turn became terminal = %#v, %v", batch, err)
	}
}

func TestReaderFailsClosedWhenSessionRewinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"session","version":3,"id":"session-1"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cursor, err := Attach(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNew(path, cursor, &memorySink{}); !errors.Is(err, ErrSessionRewound) {
		t.Fatalf("rewind error = %v", err)
	}
}

func appendBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
