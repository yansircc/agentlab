package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func TestCLIWorker(t *testing.T) {
	if os.Getenv("AGENTLAB_CLI_HELPER") != "1" {
		return
	}
	fmt.Println(`{"type":"message","text":"working"}`)
	fmt.Println(`{"type":"result","contract":"agentlab.worker-result.v1","outcome":"success","value":{"ok":true}}`)
	os.Exit(0)
}

func TestOwnedCLIStartStatusAndInspect(t *testing.T) {
	root := t.TempDir()
	bindCLIRunManifest(t, root, "cli-experiment", "owned-cli")
	request := writeJSONFile(t, t.TempDir(), "start.json", map[string]any{
		"public_command":     []string{os.Args[0], "-test.run=TestCLIWorker"},
		"public_environment": map[string]string{"AGENTLAB_CLI_HELPER": "1"},
	})
	result, err := dispatch([]string{"run", "start", "-root", root, "-experiment", "cli-experiment", "-run", "owned-cli", "-request", request, "-first-event", "1s", "-soft-idle", "2s", "-hard-idle", "3s"})
	if err != nil {
		t.Fatal(err)
	}
	started, ok := result.(run.StartResult)
	if !ok || started.RunID != "owned-cli" || started.Code != 0 {
		t.Fatalf("start result = %#v", result)
	}
	statusResult, err := dispatch([]string{"run", "status", "-root", root, "-experiment", "cli-experiment", "-run", "owned-cli"})
	if err != nil {
		t.Fatal(err)
	}
	projection, ok := statusResult.(run.OperationStatusProjection)
	if !ok || projection.Status.Health != run.HealthExitedClean {
		t.Fatalf("status = %#v", statusResult)
	}
	page, err := dispatch([]string{"inspect", "-root", root, "-experiment", "cli-experiment", "-run", "owned-cli", "-after", "0", "-limit", "2"})
	records, ok := page.([]ledger.Record)
	if err != nil || !ok || len(records) != 2 {
		t.Fatalf("inspect = %#v, %v", page, err)
	}
}

func TestAttachedCLIStopIsDurableWithoutClaimingProcessDeath(t *testing.T) {
	root := t.TempDir()
	bindCLIRunManifest(t, root, "cli-experiment", "attached-cli")
	session := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(session, []byte(`{"type":"session","version":3,"id":"cli-session"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatch([]string{
		"run", "attach", "begin", "-root", root, "-experiment", "cli-experiment", "-run", "attached-cli", "-adapter", "pi", "-stream", session,
		"-first-event", "1s", "-soft-idle", "2s", "-hard-idle", "3s",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatch([]string{"run", "stop", "-root", root, "-experiment", "cli-experiment", "-run", "attached-cli", "-reason", "acceptance"}); err != nil {
		t.Fatal(err)
	}
	statusResult, err := dispatch([]string{"run", "status", "-root", root, "-experiment", "cli-experiment", "-run", "attached-cli"})
	if err != nil {
		t.Fatal(err)
	}
	projection := statusResult.(run.OperationStatusProjection)
	if projection.Status.Health != run.HealthAbandoned || projection.Status.ProcessLiveness != run.ProcessUnknown || !projection.Status.StopRequested {
		t.Fatalf("attached status = %#v", projection.Status)
	}
}

func TestCLIExperimentBindRunRejectsRawInputs(t *testing.T) {
	request := writeJSONFile(t, t.TempDir(), "raw-inputs.json", map[string]any{
		"experiment_id": "cli-experiment", "run_id": "worker", "origin": experiment.NewFreshOrigin(), "inputs": map[string]any{},
	})
	if _, err := dispatch([]string{"experiment", "bind-run", "-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("CLI accepted raw manifest inputs")
	}
}

func bindCLIRunManifest(t *testing.T, root, experimentID, runID string) {
	t.Helper()
	prepID := experimentID + "-prep"
	prep, _ := preparation.Open(root, prepID)
	if status, _ := prep.Status(); status.Phase != preparation.PhaseSealed {
		_, _ = prep.Begin(preparation.BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "source.txt", Content: []byte("source")}}, Authority: "designer"})
		recordTestLeakageAssay(t, root, prep)
		basis, _ := prep.ChallengeBasis()
		_ = prep.Challenge(preparation.Challenge{Basis: basis})
		_, _ = prep.Seal()
	}
	experimentOperation, _ := experiment.Open(root, experimentID)
	_, _ = experimentOperation.Begin(prepID)
	bindExistingExperimentRun(t, root, experimentOperation, runID)
}

func recordTestLeakageAssay(t *testing.T, root string, prep *preparation.Operation) {
	t.Helper()
	status, err := prep.Status()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := artifact.NewStore(filepath.Join(root, "artifacts")).Put([]byte("independent semantic leakage assay"))
	if err != nil {
		t.Fatal(err)
	}
	if err := prep.RecordLeakageAssay(preparation.LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer-1", Authority: "reviewer",
		Method: "semantic-contrast-review", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{evidence},
	}); err != nil {
		t.Fatal(err)
	}
}

func bindExistingExperimentRun(t *testing.T, root string, operation *experiment.Operation, runID string) {
	t.Helper()
	store := artifact.NewStore(filepath.Join(root, "artifacts"))
	put := func(name string) artifact.Ref {
		ref, err := store.Put([]byte(runID + ":" + name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	fixture := put("fixture")
	candidate, err := source.Build(store, []source.InputFile{{Path: "main.go", Content: []byte("package candidate\n")}})
	if err != nil {
		t.Fatal(err)
	}
	reset, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{
		Contract: experiment.FixtureResetContract, RunID: runID, Fixture: fixture,
		Baseline: put("fixture-baseline"), Evidence: []artifact.Ref{put("fixture-reset-evidence")},
	})
	if err != nil {
		t.Fatal(err)
	}
	inputs := experiment.RunInputs{
		Harness: put("harness"), Trial: put("trial"), Candidate: candidate, Adapter: put("adapter"),
		OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset, EvidencePolicy: put("evidence"),
		StopPolicy: put("stop"), WorkerRuntime: put("runtime"), Environment: put("environment"),
	}
	prepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: runID, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	_, err = operation.BindPreparedRun(runID, experiment.NewFreshOrigin(), prepared)
	if err != nil {
		t.Fatal(err)
	}
}

func completeExistingCandidate(t *testing.T, root string, operation *experiment.Operation, runID string, handoff, sourceRef, candidate artifact.Ref, decision experiment.SupervisorDecision) artifact.Ref {
	t.Helper()
	bindExistingExperimentRun(t, root, operation, runID)
	store := artifact.NewStore(filepath.Join(root, "artifacts"))
	put := func(name string) artifact.Ref {
		ref, err := store.Put([]byte(runID + ":" + name))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	profile := run.CoderProfile{Handoff: handoff, SourceSnapshot: sourceRef, CandidateWorkspace: put("workspace"), CapabilityProfile: put("capability")}
	payload, err := run.EncodeStartPayload(effect.CoderStart, run.StartPayload{Coder: &profile})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := store.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: decision.ID, RunID: runID, Kind: effect.CoderStart, Payload: payloadRef}
	if err := operation.CommitDecisionBoundEffect(experiment.DecisionBoundEffect{Decision: decision, Intent: intent}); err != nil {
		t.Fatal(err)
	}
	coder, err := run.Open(root, "review-exp", runID)
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	_, err = coder.BeginManagedAttachedEffect(intent, run.ManagedAttachedSpec{
		Adapter: "test", Policy: policy, Capabilities: run.RequiredAdapterCapabilities(), Command: []string{"/bin/sh", "-c", "sleep 0.05"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: root,
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
	deadline := time.Now().Add(time.Second)
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
