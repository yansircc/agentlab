package pi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func TestOperationPollPersistsCursorWithSanitizedBatch(t *testing.T) {
	dir := t.TempDir()
	session := filepath.Join(dir, "pi-session.jsonl")
	header := `{"type":"session","version":3,"id":"pi-session-1","timestamp":"2026-08-03T00:00:00Z","cwd":"/tmp"}` + "\n"
	if err := os.WriteFile(session, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "agentlab")
	op, err := run.Open(root, "test-experiment", "attached")
	if err != nil {
		t.Fatal(err)
	}
	bindAdapterTestManifest(t, root, "test-experiment", "attached")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := Begin(op, session, policy, nil); err != nil {
		t.Fatal(err)
	}
	status, err := op.StatusAt(fixedUnknownProber{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if status.Health != run.HealthUnverifiable || status.ProcessLiveness != run.ProcessUnknown {
		t.Fatalf("attached status = %#v", status)
	}
	if status.Adapter == nil || status.Adapter.StreamID != "pi-session-1" || status.Deadlines.FirstEvent != nil {
		t.Fatalf("attached observable facts = %#v", status)
	}

	secret := "PI_PRIVATE_SENTINEL"
	assistant := `{"type":"message","id":"assistant-1","parentId":null,"timestamp":"2026-08-03T00:00:01Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"` + secret + `"},{"type":"text","text":"public"},{"type":"toolCall","id":"call-1","name":"read","arguments":{"path":"README.md"}}]}}` + "\n"
	partialResult := `{"type":"message","id":"result-1","parentId":"assistant-1","timestamp":"2026-08-03T00:00:02Z","message":{"role":"toolResult","toolCallId":"call-1","toolName":"read","content":[{"type":"text","text":"ok"}],"isError":false}}`
	appendBytes(t, session, []byte(assistant+partialResult))
	first, err := Poll(op, session)
	if err != nil {
		t.Fatal(err)
	}
	if first.BatchCount != 1 || first.EventCount != 2 || first.Excluded != 1 {
		t.Fatalf("first poll = %#v", first)
	}
	assertAbsentFromTree(t, root, secret)

	appendBytes(t, session, []byte("\n"))
	reopened, _ := run.Open(root, "test-experiment", "attached")
	second, err := Poll(reopened, session)
	if err != nil {
		t.Fatal(err)
	}
	if second.BatchCount != 1 || second.EventCount != 1 {
		t.Fatalf("restart poll = %#v", second)
	}
	third, err := Poll(reopened, session)
	if err != nil {
		t.Fatal(err)
	}
	if third.BatchCount != 0 || third.EventCount != 0 || third.Offset != second.Offset {
		t.Fatalf("duplicate poll = %#v", third)
	}
	records, err := reopened.Inspect(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[1].Kind != "adapter_batch" || records[2].Kind != "adapter_batch" {
		t.Fatalf("ledger records = %#v", records)
	}
	if _, err := reopened.RequestStop("test_stop"); err != nil {
		t.Fatal(err)
	}
	appendBytes(t, session, []byte(`{"type":"message","id":"after-stop","parentId":"result-1","timestamp":"2026-08-03T00:00:03Z","message":{"role":"user","content":"must not be captured"}}`+"\n"))
	stopped, err := Poll(reopened, session)
	if err != nil {
		t.Fatal(err)
	}
	if !stopped.Stopped || stopped.BatchCount != 0 || stopped.Offset != third.Offset {
		t.Fatalf("poll after stop = %#v", stopped)
	}
}

func bindAdapterTestManifest(t *testing.T, root, experimentID, runID string) {
	t.Helper()
	prep, _ := preparation.Open(root, experimentID+"-preparation")
	_, _ = prep.Begin(preparation.BeginSpec{UserIntent: []byte("Pi task"), SourceFiles: []source.InputFile{{Path: "source.txt", Content: []byte("source")}}, Authority: "designer"})
	status, _ := prep.Status()
	store := artifact.NewStore(filepath.Join(root, "artifacts"))
	evidence, _ := store.Put([]byte("independent leakage assay"))
	_ = prep.RecordLeakageAssay(preparation.LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer-1", Authority: "reviewer",
		Method: "semantic-contrast-review", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{evidence},
	})
	basis, _ := prep.ChallengeBasis()
	_ = prep.Challenge(preparation.Challenge{Basis: basis})
	_, _ = prep.Seal()
	operation, _ := experiment.Open(root, experimentID)
	if _, err := operation.Begin(experimentID + "-preparation"); err != nil {
		t.Fatal(err)
	}
	put := func(name string) artifact.Ref {
		ref, err := store.Put([]byte(name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	fixture := put("fixture")
	reset, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{
		Contract: experiment.FixtureResetContract, RunID: runID, Fixture: fixture,
		Baseline: put("fixture-baseline"), Evidence: []artifact.Ref{put("fixture-reset-evidence")},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = operation.BindRun(runID, experiment.NewFreshOrigin(), experiment.RunInputs{
		Harness: put("harness"), Trial: put("trial"), Candidate: put("candidate"), Adapter: put("pi-adapter"),
		OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset, EvidencePolicy: put("evidence"), StopPolicy: put("stop"),
		WorkerRuntime: put("pi-runtime"), Environment: put("environment"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

type fixedUnknownProber struct{}

func (fixedUnknownProber) Observe(processidentity.Identity) processidentity.Observation {
	return processidentity.Unknown
}

func assertAbsentFromTree(t *testing.T, root, forbidden string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("private thinking persisted in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
