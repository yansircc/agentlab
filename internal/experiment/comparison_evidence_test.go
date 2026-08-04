package experiment

import (
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func TestComparisonManifestsRequireVerifiedTerminalOracleEvidence(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "prep")
	op, err := Open(root, "comparison-exp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.Begin("prep"); err != nil {
		t.Fatal(err)
	}

	bindTestRun(t, op, "unstarted")
	if _, err := op.comparisonManifests([]string{"unstarted"}); err == nil || !strings.Contains(err.Error(), "decision-bound Worker start") {
		t.Fatalf("unstarted comparison error = %v", err)
	}

	bindTestRun(t, op, "unfinished")
	startComparisonWorkerWithoutTerminal(t, op, "unfinished")
	if _, err := op.comparisonManifests([]string{"unfinished"}); err == nil || !strings.Contains(err.Error(), "accepted terminal") {
		t.Fatalf("unfinished comparison error = %v", err)
	}

	bindTestRun(t, op, "no-oracle")
	completeComparisonWorker(t, op, "no-oracle", nil)
	if _, err := op.comparisonManifests([]string{"no-oracle"}); err == nil || !strings.Contains(err.Error(), "objective oracle") {
		t.Fatalf("missing oracle comparison error = %v", err)
	}

	bindTestRun(t, op, "verified")
	worker := completeComparisonWorker(t, op, "verified", []comparison.OracleClaim{{ID: "target-owner", Passed: true, HeldOut: true}})
	identities, err := op.comparisonManifests([]string{"verified"})
	if err != nil {
		t.Fatal(err)
	}
	identity := identities["verified"]
	if !identity.StartVerified || !identity.TerminalAccepted || !identity.OracleEvidence.Valid() || len(identity.OracleClaims) != 1 {
		t.Fatalf("comparison identity = %#v", identity)
	}
	manifest, _, err := op.RunManifest("verified")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Candidate = testCandidate(t, op, "mismatched-oracle-candidate")
	if _, _, err := op.comparisonOracleEvidence("verified", manifest, worker); err == nil || !strings.Contains(err.Error(), "differs from run manifest") {
		t.Fatalf("mismatched oracle error = %v", err)
	}
	if accepted, err := worker.TerminalAccepted(); err != nil || !accepted {
		t.Fatalf("comparison Worker terminal = %v, %v", accepted, err)
	}
}

func startComparisonWorkerWithoutTerminal(t *testing.T, operation *Operation, runID string) *run.Operation {
	t.Helper()
	payload, err := run.EncodeStartPayload(effect.WorkerStart, run.StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "unfinished-worker-start-" + runID, RunID: runID, Kind: effect.WorkerStart, Payload: payloadRef}
	if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: SupervisorDecision{
		ID: intent.ID, WorkerRun: runID, Claim: "the fresh Worker run starts", Action: DecisionWorkerStart,
		Falsifier: "the Worker starts without a decision-bound effect",
	}, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	worker, err := run.Open(operation.root, operation.id, runID)
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := worker.BeginAttachedEffect(intent, run.AttachedSpec{
		Adapter: "comparison-unfinished", StreamID: "unfinished-" + runID, InitialCursor: []byte("cursor-0"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities(),
	}); err != nil {
		t.Fatal(err)
	}
	return worker
}
