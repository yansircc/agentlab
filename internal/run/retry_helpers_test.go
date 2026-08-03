package run

import (
	"os"
	"path/filepath"
	"testing"
)

func readAttempts(t *testing.T, op *Operation) []attemptState {
	t.Helper()
	root := filepath.Join(op.dir, "launch-attempts")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	states := make([]attemptState, 0, len(entries))
	for _, entry := range entries {
		attempt := &launchAttempt{id: entry.Name(), log: ledgerForAttempt(root, entry.Name())}
		state, err := attempt.state()
		if err != nil {
			t.Fatal(err)
		}
		states = append(states, state)
	}
	return states
}

func onlyAttemptID(t *testing.T, op *Operation) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(op.dir, "launch-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(entries))
	}
	return entries[0].Name()
}
