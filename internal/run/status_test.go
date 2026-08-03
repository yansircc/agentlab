package run

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/processidentity"
)

type fixedProber processidentity.Observation

func (p fixedProber) Observe(processidentity.Identity) processidentity.Observation {
	return processidentity.Observation(p)
}

func TestStatusIsReplayableAndIdentityAware(t *testing.T) {
	root := t.TempDir()
	op, err := Open(root, "test-experiment", "status-test")
	if err != nil {
		t.Fatal(err)
	}
	identity := processidentity.Identity{PID: 42, PGID: 42, StartToken: "A", CommandHash: "hash", Executable: "worker"}
	manifest := bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: 100 * time.Millisecond, SoftIdleTimeout: 100 * time.Millisecond, HardIdleTimeout: 2 * time.Second, OwnsWorkerProcess: true}
	if _, err := op.ledger.Append(time.Unix(1, 0), eventProcessStarted, processStarted{AttemptID: "test-attempt", Manifest: manifest, Process: processHandle{Kind: processOwned, Identity: &identity}, Policy: policy}); err != nil {
		t.Fatal(err)
	}
	first, err := artifact.NewStore(root + "/artifacts").Put([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.ledger.Append(time.Unix(2, 0), eventEvidence, evidence{Stream: "stdout", Label: "public_output", Raw: first}); err != nil {
		t.Fatal(err)
	}

	reopened, _ := Open(root, "test-experiment", "status-test")
	active, err := reopened.StatusAt(fixedProber(processidentity.Matches), time.Unix(2, int64(50*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if active.Health != HealthAliveActive || active.ProcessLiveness != ProcessAlive || active.StreamActivity != RecentEvent {
		t.Fatalf("matching status = %#v", active)
	}
	if active.ProcessIdentity == nil || active.ProcessIdentity.StartToken != identity.StartToken || active.Deadlines.FirstEvent == nil || active.Deadlines.SoftIdle == nil || active.Deadlines.HardIdle == nil {
		t.Fatalf("observable identity/deadlines = %#v", active)
	}
	abandoned, err := reopened.StatusAt(fixedProber(processidentity.Mismatch), time.Unix(2, int64(50*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if abandoned.Health != HealthAbandoned {
		t.Fatalf("mismatched status = %#v", abandoned)
	}
	unverifiable, err := reopened.StatusAt(fixedProber(processidentity.Unknown), time.Unix(2, int64(50*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if unverifiable.Health != HealthUnverifiable {
		t.Fatalf("unknown status = %#v", unverifiable)
	}
	if _, err := op.ledger.Append(time.Unix(3, 0), eventSoftIdle, struct{}{}); err != nil {
		t.Fatal(err)
	}
	silent, err := reopened.StatusAt(fixedProber(processidentity.Matches), time.Unix(3, int64(150*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if silent.Health != HealthAliveSilent || silent.StreamActivity != SoftIdle {
		t.Fatalf("silent status = %#v", silent)
	}
	second, err := artifact.NewStore(root + "/artifacts").Put([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.ledger.Append(time.Unix(5, 0), eventEvidence, evidence{Stream: "stdout", Label: "public_output", Raw: second}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.ledger.Append(time.Unix(5, int64(10*time.Millisecond)), eventNoProgress, progressFact{Detector: "test-information-gain"}); err != nil {
		t.Fatal(err)
	}
	noProgress, err := reopened.StatusAt(fixedProber(processidentity.Matches), time.Unix(5, int64(50*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if noProgress.Health != HealthAliveNoProgress || noProgress.SemanticProgress != NoProgressEvidence {
		t.Fatalf("no-progress status = %#v", noProgress)
	}
}
