package run

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/processidentity"
)

func TestStatusProjectionIsDisposableAndRebuildable(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "test-experiment", "projection")
	manifest := bindTestManifest(t, op)
	identity := processidentity.Identity{PID: 42, PGID: 42, StartToken: "A", CommandHash: "hash", Executable: "worker"}
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	if _, err := op.ledger.Append(time.Unix(1, 0), eventProcessStarted, processStarted{AttemptID: "test-attempt", Manifest: manifest, Process: processHandle{Kind: processOwned, Identity: &identity}, Policy: policy}); err != nil {
		t.Fatal(err)
	}
	ref := artifact.Ref{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if _, err := op.ledger.Append(time.Unix(2, 0), eventEvidence, evidence{Stream: "stdout", Label: "public_output", Raw: ref}); err != nil {
		t.Fatal(err)
	}
	asOf := time.Unix(2, int64(500*time.Millisecond)).UTC()
	if _, err := op.ProjectStatusAt(fixedProber(processidentity.Matches), asOf); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "experiments", "test-experiment", "runs", "projection", "result.json")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("corrupt cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := op.ProjectStatusAt(fixedProber(processidentity.Matches), asOf); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("projection changed after rebuild:\n%s\n%s", first, second)
	}
}
