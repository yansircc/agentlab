package experiment

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func sealPreparation(t *testing.T, root, id string) {
	t.Helper()
	prep, _ := preparation.Open(root, id)
	if _, err := prep.Begin(preparation.BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "owner.go", Content: []byte("package owner\n\nfunc transition() {}\n")}}, Authority: "designer"}); err != nil {
		t.Fatal(err)
	}
	status, _ := prep.Status()
	evidence, _ := artifact.NewStore(filepath.Join(root, "artifacts")).Put([]byte("independent leakage assay"))
	if err := prep.RecordLeakageAssay(preparation.LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer-1", Authority: "reviewer",
		Method: "semantic-contrast-review", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{evidence},
	}); err != nil {
		t.Fatal(err)
	}
	basis, _ := prep.ChallengeBasis()
	if err := prep.Challenge(preparation.Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Seal(); err != nil {
		t.Fatal(err)
	}
}

func attachedRunWithEvidence(t *testing.T, root, experimentID, runID string) *run.Operation {
	t.Helper()
	op, _ := run.Open(root, experimentID, runID)
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor-0"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	writer, _, err := op.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Commit([]byte("cursor-1"), run.AdapterBatch{Events: []run.AdapterEvent{
		{Kind: "tool_result", Raw: []byte("first"), Label: "validation_failure"},
		{Kind: "tool_result", Raw: []byte("second"), Label: "validation_failure"},
	}})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func bindTestRun(t *testing.T, operation *Operation, runID string) {
	t.Helper()
	put := func(name string) artifact.Ref {
		ref, err := operation.artifacts.Put([]byte(name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	fixture := put("fixture")
	reset := recordTestFixtureReset(t, operation, runID, fixture, put("fixture-baseline"))
	bindPreparedTestRun(t, operation, runID, NewFreshOrigin(), RunInputs{
		Harness: put("harness"), Trial: put("trial"), Candidate: testCandidate(t, operation, "baseline"), Adapter: put("adapter"),
		OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset, EvidencePolicy: put("evidence-policy"),
		StopPolicy: put("stop-policy"), WorkerRuntime: put("runtime"), Environment: put("environment"),
	})
}

func bindPreparedTestRun(t *testing.T, operation *Operation, runID string, origin RunOrigin, inputs RunInputs) artifact.Ref {
	t.Helper()
	prepared, err := RecordPreparedRun(operation.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: runID, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := operation.BindPreparedRun(runID, origin, prepared)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func testCandidate(t *testing.T, operation *Operation, value string) artifact.Ref {
	t.Helper()
	ref, err := source.Build(operation.artifacts, []source.InputFile{{Path: "main.go", Content: []byte("package candidate\n// " + value + "\n")}})
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func recordTestFixtureReset(t *testing.T, operation *Operation, runID string, fixture, baseline artifact.Ref) artifact.Ref {
	t.Helper()
	evidence, err := operation.artifacts.Put([]byte("reset-evidence:" + runID))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := RecordFixtureReset(operation.artifacts, FixtureResetProof{
		Contract: FixtureResetContract, RunID: runID, Fixture: fixture, Baseline: baseline, Evidence: []artifact.Ref{evidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func rebindTestFixtureReset(t *testing.T, operation *Operation, runID string, inputs RunInputs) RunInputs {
	t.Helper()
	previous, err := loadFixtureReset(operation.artifacts, inputs.FixtureReset)
	if err != nil {
		t.Fatal(err)
	}
	inputs.FixtureReset = recordTestFixtureReset(t, operation, runID, inputs.Fixture, previous.Baseline)
	return inputs
}

// completeComparisonWorker models the Host-owned facts needed for a fresh
// autonomous comparison: a decision-bound Worker start, an accepted terminal,
// and one adapter-admitted oracle artifact bound to the exact manifest.
func completeComparisonWorker(t *testing.T, operation *Operation, runID string, claims []comparison.OracleClaim) *run.Operation {
	t.Helper()
	manifest, _, err := operation.RunManifest(runID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := run.EncodeStartPayload(effect.WorkerStart, run.StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "worker-start-" + runID, RunID: runID, Kind: effect.WorkerStart, Payload: payloadRef}
	if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: SupervisorDecision{
		ID: intent.ID, WorkerRun: runID, Claim: "the sealed fresh Worker run must start", Action: DecisionWorkerStart,
		Falsifier: "the Worker starts without its decision-bound effect",
	}, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	worker, err := run.Open(operation.root, operation.id, runID)
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	if _, err := worker.BeginManagedAttachedEffect(intent, run.ManagedAttachedSpec{
		Adapter: "comparison-test", Policy: policy, Capabilities: run.RequiredAdapterCapabilities(), Command: []string{"/bin/sh", "-c", "sleep 0.2"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: operation.root,
		Ready: func() (string, []byte, error) { return "worker-session-" + runID, []byte("cursor-0"), nil }, Finalize: func(code int) error {
			if code != 0 {
				return errors.New("comparison Worker exited unsuccessfully")
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if claims != nil {
		data, err := comparison.EncodeOracleEvidence(comparison.OracleEvidence{
			Contract: comparison.OracleEvidenceContract, RunID: runID, Candidate: manifest.Candidate, Trial: manifest.Trial, OracleSet: manifest.OracleSet, Claims: claims,
		})
		if err != nil {
			t.Fatal(err)
		}
		var writer *run.AdapterWriter
		// A healthy managed Worker reaches its accepted terminal shortly after the
		// process exits, but the durable event appends run under -race on shared CI
		// runners; a one-second poll is too tight there. Five seconds still fails
		// fast for a genuinely broken terminal path while tolerating slow-but-
		// correct hosts.
		writerDeadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(writerDeadline) {
			writer, _, err = worker.AcquireAdapterWriter("comparison-test")
			if err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil || writer == nil {
			t.Fatal(err)
		}
		err = writer.Commit([]byte("cursor-1"), run.AdapterBatch{Events: []run.AdapterEvent{
			{Kind: run.EvidenceToolResult, Label: "validation_failure", Raw: []byte("first")},
			{Kind: run.EvidenceToolResult, Label: "validation_failure", Raw: []byte("second")},
			{Kind: run.EvidenceOracle, Label: "objective_oracle", Raw: data},
		}})
		if closeErr := writer.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	terminalDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(terminalDeadline) {
		accepted, err := worker.TerminalAccepted()
		if err != nil {
			t.Fatal(err)
		}
		if accepted {
			return worker
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("comparison Worker did not produce an accepted terminal result")
	return nil
}

func completeCandidate(t *testing.T, operation *Operation, runID string, handoff, candidate artifact.Ref) artifact.Ref {
	return completeCandidateWithStart(t, operation, runID, handoff, candidate, true)
}

func completeCandidateWithoutDecision(t *testing.T, operation *Operation, runID string, handoff, candidate artifact.Ref) artifact.Ref {
	return completeCandidateWithStart(t, operation, runID, handoff, candidate, false)
}

func completeCandidateWithStart(t *testing.T, operation *Operation, runID string, handoff, candidate artifact.Ref, decisionBound bool) artifact.Ref {
	t.Helper()
	bindTestRun(t, operation, runID)
	current, err := operation.current()
	if err != nil {
		t.Fatal(err)
	}
	put := func(value string) artifact.Ref {
		ref, err := operation.artifacts.Put([]byte(value + ":" + runID))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	profile := run.CoderProfile{Handoff: handoff, SourceSnapshot: current.begun.Source, CandidateWorkspace: put("workspace"), CapabilityProfile: put("capability")}
	payload, err := run.EncodeStartPayload(effect.CoderStart, run.StartPayload{Coder: &profile})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := operation.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	var evidence run.EvidenceRef
	for _, finding := range current.findings {
		if len(finding.Evidence) > 0 {
			evidence = finding.Evidence[0]
			break
		}
	}
	if evidence.Sequence == 0 {
		t.Fatal("test Coder requires public finding evidence")
	}
	intent := effect.Intent{ID: "coder-start-" + runID, RunID: runID, Kind: effect.CoderStart, Payload: payloadRef}
	if decisionBound {
		if err := operation.CommitDecisionBoundEffect(DecisionBoundEffect{Decision: SupervisorDecision{
			ID: intent.ID, WorkerRun: evidence.RunID, EvidenceThrough: evidence.Sequence, Claim: "Coder receives the bounded handoff", Action: DecisionCoderStart,
			Evidence: []run.EvidenceRef{evidence}, Falsifier: "Coder start omits the experiment-owned handoff",
		}, Intent: intent}); err != nil {
			t.Fatal(err)
		}
	}
	coder, err := run.Open(operation.root, operation.id, runID)
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	_, err = coder.BeginManagedAttachedEffect(intent, run.ManagedAttachedSpec{
		Adapter: "test", Policy: policy, Capabilities: run.RequiredAdapterCapabilities(), Command: []string{"/bin/sh", "-c", "sleep 0.05"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: operation.root,
		Ready: func() (string, []byte, error) { return "coder-session-" + runID, []byte("cursor"), nil }, Coder: &profile,
		Finalize: func(code int) error {
			if code != 0 {
				return errors.New("test Coder exited unsuccessfully")
			}
			_, err := coder.RecordCoderCompletion(candidate)
			return err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := coder.VerifyStartEffect(intent); err != nil {
		t.Fatalf("verify Coder start effect: %v", err)
	}
	// The Coder completion receipt is admitted by the same durable managed
	// completion path; keep the same CI-load headroom as the Worker poll above.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		receipt, _, err := coder.CoderCompletionReceipt()
		if err == nil {
			return receipt
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Coder completion receipt was not admitted")
	return artifact.Ref{}
}
