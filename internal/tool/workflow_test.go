package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

// TestFourToolsReachFullSupervisionWorkflow proves the provider-facing
// surface can reach the complete deterministic development loop once the
// Host has sealed preparation and issued PreparedRuns. The test deliberately
// invokes every Supervisor operation through Decode and Execute: no model
// input contains a path, raw session, candidate, or individual run input.
func TestFourToolsReachFullSupervisionWorkflow(t *testing.T) {
	root := t.TempDir()
	store := artifact.NewStore(root + "/artifacts")
	prep, err := preparation.Open(root, "prep")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Begin(preparation.BeginSpec{
		UserIntent:  []byte("repair the public target contract"),
		SourceFiles: []source.InputFile{{Path: "owner.go", Content: []byte("package owner\n\nfunc Target() {}\n")}},
		Authority:   "host",
	}); err != nil {
		t.Fatal(err)
	}
	sealed := sealWorkflowPreparation(t, prep, store)

	binding := Binding{Root: root, PreparationID: "prep", ExperimentID: "exp"}
	workflowInvoke(t, binding, ApplyTool, emptyApply{Action: "begin_experiment"})
	experimentOp, err := experiment.Open(root, "exp")
	if err != nil {
		t.Fatal(err)
	}
	baselinePrepared := workflowPreparedRun(t, store, "baseline", sealed.Source, "baseline")
	if _, err := experimentOp.BindPreparedRun("baseline", experiment.NewFreshOrigin(), baselinePrepared); err != nil {
		t.Fatal(err)
	}

	repaired, err := source.Build(store, []source.InputFile{{Path: "owner.go", Content: []byte("package owner\n\nfunc Target() { /* repaired */ }\n")}})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := store.Put([]byte("host-bound Coder workspace"))
	if err != nil {
		t.Fatal(err)
	}
	capability, err := store.Put([]byte("host-bound Coder capability"))
	if err != nil {
		t.Fatal(err)
	}
	runtime := &workflowRuntime{
		candidate:  repaired,
		source:     sealed.Source,
		workspace:  workspace,
		capability: capability,
		roles: map[string]workflowRole{
			"baseline-runtime": {runID: "baseline", kind: effect.WorkerStart},
			"coder-runtime":    {runID: "coder", kind: effect.CoderStart},
			"child-runtime":    {runID: "child", kind: effect.WorkerStart},
		},
		polled:          map[string]bool{},
		forkCheckpoints: map[string]artifact.Ref{},
	}
	binding.Runtime = runtime

	workflowInvoke(t, binding, RunTool, startRun{
		Action: "start", EffectID: "baseline-start", RunID: "baseline", RuntimeRef: "baseline-runtime",
		Decision: workflowDecision("baseline-start", "baseline", run.EvidenceRef{}, experiment.DecisionWorkerStart),
	})
	workflowInvoke(t, binding, RunTool, pollRun{Action: "poll", RunID: "baseline", RuntimeRef: "baseline-runtime"})
	baselineEvidence := workflowEvidence(t, root, "exp", "baseline")

	for _, value := range []inspectOperation{
		{Scope: "preparation", After: pointer(uint64(0)), Limit: 10},
		{Scope: "experiment", After: pointer(uint64(0)), Limit: 10},
		{Scope: "run", RunID: "baseline", After: pointer(uint64(0)), Limit: 10},
		{Scope: "runtime_tree", RunID: "baseline", After: pointer(uint64(0)), Limit: 10},
	} {
		workflowInvoke(t, binding, InspectTool, value)
	}
	workflowInvoke(t, binding, RunTool, statusRun{Action: "status", RunID: "baseline"})
	workflowInvoke(t, binding, ApplyTool, continueRun{Action: "continue", Value: experiment.DecisionBoundContinue{
		Decision: workflowDecision("continue-observation", "baseline", baselineEvidence, experiment.DecisionContinue),
	}})

	workflowInvoke(t, binding, RunTool, stopRun{
		Action: "stop", EffectID: "stop-baseline", RunID: "baseline", Reason: "failure is irreversible",
		Decision: workflowDecision("stop-baseline", "baseline", baselineEvidence, experiment.DecisionStop),
	})

	failure := finding.Finding{
		ID: "target-mismatch", Class: "target_mismatch", Severity: finding.SeverityHigh,
		Symptom: "receipt target differs from observed target", Impact: "deployment is unsafe", Evidence: []run.EvidenceRef{baselineEvidence},
		Confidence: finding.ConfidenceHigh, Falsifier: "target and receipt agree",
	}
	workflowInvoke(t, binding, ApplyTool, recordFinding{Action: "record_finding", Value: experiment.DecisionBoundFinding{
		Decision: workflowDecision("record-finding", "baseline", baselineEvidence, experiment.DecisionFinding), Finding: failure,
	}})
	handoffResult := workflowInvoke(t, binding, ApplyTool, renderHandoff{
		Action: "render_handoff", Decision: workflowDecision("render-handoff", "baseline", baselineEvidence, experiment.DecisionHandoff), FindingIDs: []string{failure.ID},
	})
	handoff, ok := handoffResult.(experiment.HandoffResult)
	if !ok {
		t.Fatalf("handoff result = %#v", handoffResult)
	}

	coderPrepared := workflowPreparedRun(t, store, "coder", sealed.Source, "coder")
	workflowInvoke(t, binding, ApplyTool, bindRun{
		Action: "bind_run", Prepared: coderPrepared, Origin: experiment.NewFreshOrigin(),
		Binding: experiment.DecisionBoundRunBinding{RunID: "coder", Decision: workflowDecision("bind-coder", "baseline", baselineEvidence, experiment.DecisionRunBinding)},
	})
	workflowInvoke(t, binding, RunTool, startRun{
		Action: "start", EffectID: "start-coder", RunID: "coder", RuntimeRef: "coder-runtime", Handoff: &handoff.Artifact,
		Decision: workflowDecision("start-coder", "baseline", baselineEvidence, experiment.DecisionCoderStart),
	})
	completion := workflowCoderCompletion(t, root, "exp", "coder")

	snapshot, err := source.Load(store, sealed.Source)
	if err != nil {
		t.Fatal(err)
	}
	diagnosed := diagnosis.Diagnosis{
		ID: "target-owner", State: diagnosis.Established, FindingIDs: []string{failure.ID}, SourceSnapshot: sealed.Source,
		SourceEvidence: []diagnosis.SourceEvidenceRef{{Path: snapshot.Files[0].Path, Artifact: snapshot.Files[0].Artifact, StartLine: 1, EndLine: 3, EstablishesOwner: true}},
		Owner:          "target constructor", RootCause: "execution re-derives target identity", Invariant: "one validated target reaches execution and receipt",
		RepairBoundary: "target constructor", ProhibitedPatches: []string{"staging special case"},
		AcceptanceClaims: []diagnosis.Claim{{ID: "target-owner", Statement: "explicit target has one owner", Falsifier: "execution reads default target"}},
	}
	workflowInvoke(t, binding, ApplyTool, recordDiagnosis{Action: "record_diagnosis", Value: experiment.DecisionBoundDiagnosis{
		Decision: workflowDecision("record-diagnosis", "baseline", baselineEvidence, experiment.DecisionDiagnosis), Diagnosis: diagnosed,
	}})
	workflowInvoke(t, binding, ApplyTool, bindCandidate{Action: "bind_candidate", Value: experiment.DecisionBoundCandidate{
		Decision: workflowDecision("bind-candidate", "baseline", baselineEvidence, experiment.DecisionCandidate),
		ID:       "repaired-candidate", DiagnosisID: diagnosed.ID, CoderRun: "coder", CompletionRef: completion,
	}})
	checkpointResult := workflowInvoke(t, binding, RunTool, checkpointRun{
		Action: "checkpoint", EffectID: "checkpoint-baseline", RunID: "baseline", RuntimeRef: "baseline-runtime", EntryLocator: "workflow:baseline:failure",
		Decision: workflowDecision("checkpoint-baseline", "baseline", baselineEvidence, experiment.DecisionCheckpoint),
	})
	checkpoint, ok := checkpointResult.(run.RuntimeCheckpointRecord)
	if !ok {
		t.Fatalf("checkpoint result = %#v", checkpointResult)
	}

	interventionResult := workflowInvoke(t, binding, ApplyTool, recordIntervention{Action: "record_intervention", Value: experiment.DecisionBoundIntervention{
		Decision:     workflowDecision("record-intervention", "baseline", baselineEvidence, experiment.DecisionIntervention),
		Intervention: experiment.Intervention{Contract: experiment.InterventionContract, Text: "observe the updated public target contract before continuing"},
	}})
	intervention, ok := interventionResult.(artifact.Ref)
	if !ok {
		t.Fatalf("intervention result = %#v", interventionResult)
	}
	origin, err := experiment.NewSpliceOrigin(experiment.SpliceOriginSpec{
		ParentRun: "baseline", ParentEvidence: baselineEvidence, RuntimeCheckpoint: checkpoint.Checkpoint, PublicPrefix: checkpoint.PublicPrefix,
		Intervention: &intervention, ReasonEvidence: []run.EvidenceRef{baselineEvidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	childPrepared := workflowPreparedRun(t, store, "child", repaired, "child")
	workflowInvoke(t, binding, ApplyTool, bindRun{
		Action: "bind_run", Prepared: childPrepared, Origin: origin,
		Binding: experiment.DecisionBoundRunBinding{RunID: "child", Decision: workflowDecision("bind-child", "baseline", baselineEvidence, experiment.DecisionRunBinding)},
	})
	workflowInvoke(t, binding, RunTool, forkRun{
		Action: "fork", EffectID: "fork-child", RunID: "baseline", RuntimeRef: "baseline-runtime", Checkpoint: checkpoint.Checkpoint, ChildRun: "child",
		Decision: workflowDecision("fork-child", "baseline", baselineEvidence, experiment.DecisionFork),
	})
	workflowInvoke(t, binding, RunTool, startRun{
		Action: "start", EffectID: "start-child", RunID: "child", RuntimeRef: "child-runtime",
		Decision: workflowDecision("start-child", "baseline", baselineEvidence, experiment.DecisionWorkerStart),
	})
	workflowInvoke(t, binding, RunTool, pollRun{Action: "poll", RunID: "child", RuntimeRef: "child-runtime"})
	childEvidence := workflowEvidence(t, root, "exp", "child")

	comparisonValue := experiment.DecisionBoundComparison{
		Decision: workflowDecision("record-comparison", "child", childEvidence, experiment.DecisionComparison),
		Observation: comparison.Observation{
			ID: "guided-comparison", CandidateID: "repaired-candidate", BaselineRuns: []string{"baseline"}, CandidateRuns: []string{"child"},
			Policy: comparison.Policy{MinimumRepetitions: 2, RequiredClaims: []string{"target-owner"}},
		},
	}
	comparisonData, err := json.Marshal(recordComparison{Action: "record", Value: comparisonValue})
	if err != nil {
		t.Fatal(err)
	}
	comparisonOperation, err := Decode(CompareTool, comparisonData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(binding, comparisonOperation); err == nil {
		t.Fatal("guided comparison without terminal oracle evidence was accepted")
	}

	blockedGate := gate.Spec{ID: "guided-gate", CandidateID: "repaired-candidate", Items: []gate.Item{{
		ID: "fresh-proof", Status: gate.Blocked, Statement: "guided run is not fresh proof", Impact: "autonomous claim remains blocked",
		Evidence: []run.EvidenceRef{childEvidence}, Severity: finding.SeverityHigh, Confidence: finding.ConfidenceHigh, Falsifier: "fresh repetitions pass",
	}}}
	gateResult := workflowInvoke(t, binding, CompareTool, recordGate{Action: "gate_record", Value: experiment.DecisionBoundGate{
		Decision: workflowDecision("record-gate", "child", childEvidence, experiment.DecisionGate), Receipt: gate.Receipt{Spec: blockedGate, Candidate: repaired},
	}})
	if result, ok := gateResult.(gate.Result); !ok || result.Verdict != gate.Block {
		t.Fatalf("guided gate = %#v", gateResult)
	}
	workflowInvoke(t, binding, CompareTool, showGate{Action: "gate_show", GateID: blockedGate.ID})

	status, err := experimentOp.Status()
	if err != nil || len(status.DecisionIDs) != 15 || len(status.InterventionRefs) != 1 || len(status.RunIDs) != 3 || len(status.CandidateIDs) != 1 || len(status.GateIDs) != 1 {
		t.Fatalf("workflow status = %#v, %v", status, err)
	}
}

type workflowRole struct {
	runID string
	kind  effect.Kind
}

type workflowRuntime struct {
	candidate, source, workspace, capability artifact.Ref
	roles                                    map[string]workflowRole
	polled                                   map[string]bool
	forkCheckpoints                          map[string]artifact.Ref
}

func (h *workflowRuntime) StartIntent(binding Binding, request StartRequest) (effect.Intent, error) {
	role, ok := h.roles[request.RuntimeRef]
	if !ok || role.runID != request.RunID || request.ID == "" || (role.kind == effect.WorkerStart && request.Handoff != nil) || (role.kind == effect.CoderStart && request.Handoff == nil) {
		return effect.Intent{}, errors.New("workflow runtime start request is invalid")
	}
	payload := run.StartPayload{}
	if role.kind == effect.CoderStart {
		payload.Coder = &run.CoderProfile{Handoff: *request.Handoff, SourceSnapshot: h.source, CandidateWorkspace: h.workspace, CapabilityProfile: h.capability}
	}
	data, err := run.EncodeStartPayload(role.kind, payload)
	if err != nil {
		return effect.Intent{}, err
	}
	ref, err := binding.store().Put(data)
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: role.kind, Payload: ref}, nil
}

func (h *workflowRuntime) CheckpointIntent(binding Binding, request CheckpointRequest) (effect.Intent, error) {
	role, ok := h.roles[request.RuntimeRef]
	if !ok || role.kind != effect.WorkerStart || role.runID != request.RunID || request.ID == "" || request.EntryLocator != "workflow:baseline:failure" {
		return effect.Intent{}, errors.New("workflow checkpoint request is invalid")
	}
	ref, err := binding.store().Put([]byte("workflow checkpoint intent"))
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: effect.Checkpoint, Payload: ref}, nil
}

func (h *workflowRuntime) ForkIntent(binding Binding, request ForkRequest) (effect.Intent, error) {
	role, ok := h.roles[request.RuntimeRef]
	if !ok || role.kind != effect.WorkerStart || role.runID != request.RunID || request.ID == "" || request.ChildRun != "child" || !request.Checkpoint.Valid() {
		return effect.Intent{}, errors.New("workflow fork request is invalid")
	}
	ref, err := binding.store().Put([]byte("workflow fork intent"))
	if err != nil {
		return effect.Intent{}, err
	}
	h.forkCheckpoints[request.ID] = request.Checkpoint
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: effect.Fork, Payload: ref}, nil
}

func (h *workflowRuntime) Start(binding Binding, intent effect.Intent, ref string) (any, error) {
	role, ok := h.roles[ref]
	if !ok || role.runID != intent.RunID || role.kind != intent.Kind {
		return nil, errors.New("workflow start binding is invalid")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	if intent.Kind == effect.WorkerStart {
		return op.BeginAttachedEffect(intent, run.AttachedSpec{Adapter: "workflow", StreamID: "session-" + intent.RunID, InitialCursor: []byte("cursor-" + intent.RunID), Policy: workflowAttachedPolicy(), Capabilities: run.RequiredAdapterCapabilities()})
	}
	profile, err := op.CoderProfile(intent)
	if err != nil {
		return nil, err
	}
	return op.BeginManagedAttachedEffect(intent, run.ManagedAttachedSpec{
		Adapter: "workflow", Policy: workflowManagedPolicy(), Capabilities: run.RequiredAdapterCapabilities(),
		Command: []string{"/bin/sh", "-c", "exit 0"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: binding.Root,
		Ready: func() (string, []byte, error) {
			return "session-" + intent.RunID, []byte("cursor-" + intent.RunID), nil
		}, Coder: &profile,
		Finalize: func(code int) error {
			if code != 0 {
				return fmt.Errorf("workflow Coder exited with %d", code)
			}
			_, err := op.RecordCoderCompletion(h.candidate)
			return err
		},
	})
}

func (h *workflowRuntime) Poll(binding Binding, runID, ref string) (any, error) {
	role, ok := h.roles[ref]
	if !ok || role.runID != runID {
		return nil, errors.New("workflow poll binding is invalid")
	}
	if h.polled[runID] {
		return map[string]any{"run_id": runID, "new_events": 0}, nil
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, runID)
	if err != nil {
		return nil, err
	}
	writer, _, err := op.AcquireAdapterWriter("workflow")
	if err != nil {
		return nil, err
	}
	defer writer.Close()
	if err := writer.Commit([]byte("next-"+runID), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: run.EvidenceToolResult, SourceLocator: "workflow:" + runID + ":failure", Label: "objective_oracle", Raw: []byte("target mismatch")}}}); err != nil {
		return nil, err
	}
	h.polled[runID] = true
	return map[string]any{"run_id": runID, "new_events": 1}, nil
}

func (h *workflowRuntime) RuntimeTree(binding Binding, runID string, after uint64, limit int) (any, error) {
	role, ok := h.roles["baseline-runtime"]
	if !ok || runID != role.runID || after != 0 || limit < 1 {
		return nil, errors.New("workflow runtime tree binding is invalid")
	}
	return map[string]any{"contract": "workflow-runtime-tree.v1", "run_id": runID, "entries": 1}, nil
}

func (h *workflowRuntime) Checkpoint(binding Binding, intent effect.Intent, ref string) (any, error) {
	if ref != "baseline-runtime" || intent.Kind != effect.Checkpoint {
		return nil, errors.New("workflow checkpoint binding is invalid")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	result, err := op.RecordRuntimeCheckpoint(intent, run.RuntimeCheckpointSpec{Adapter: "workflow", Session: []byte("baseline-session"), OpaqueState: []byte("baseline-state"), PublicPrefix: []byte("baseline-public-prefix")})
	if err != nil {
		return nil, err
	}
	evidence := []byte("workflow checkpoint receipt")
	if err := op.RecordEffectObservation(intent, evidence); err != nil {
		return nil, err
	}
	if _, err := op.SettleEffect(intent, evidence); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *workflowRuntime) Fork(binding Binding, intent effect.Intent, ref, childRun string) (any, error) {
	checkpoint, ok := h.forkCheckpoints[intent.ID]
	if ref != "baseline-runtime" || intent.Kind != effect.Fork || childRun != "child" || !ok {
		return nil, errors.New("workflow fork binding is invalid")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	prefix, err := op.RuntimeCheckpointPublicPrefix(checkpoint)
	if err != nil {
		return nil, err
	}
	prefixData, err := binding.store().Read(prefix)
	if err != nil {
		return nil, err
	}
	result, err := op.RecordSessionForked(intent, run.SessionForkSpec{ExpectedCheckpoint: checkpoint, ChildSession: []byte("child-session"), ObservedPrefix: prefixData, AdapterIdentity: []byte("workflow-adapter")})
	if err != nil {
		return nil, err
	}
	evidence, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err := op.RecordEffectObservation(intent, evidence); err != nil {
		return nil, err
	}
	if _, err := op.SettleEffect(intent, evidence); err != nil {
		return nil, err
	}
	return result, nil
}

func workflowAttachedPolicy() run.StopPolicy {
	return run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
}

func workflowManagedPolicy() run.StopPolicy {
	policy := workflowAttachedPolicy()
	policy.OwnsWorkerProcess = true
	return policy
}

func workflowInvoke(t *testing.T, binding Binding, toolName string, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := Decode(toolName, data)
	if err != nil {
		t.Fatalf("decode %s %s: %v", toolName, data, err)
	}
	result, err := Execute(binding, operation)
	if err != nil {
		t.Fatalf("execute %s %s: %v", toolName, data, err)
	}
	return result
}

func workflowDecision(id, workerRun string, evidence run.EvidenceRef, action experiment.DecisionAction) experiment.SupervisorDecision {
	decision := experiment.SupervisorDecision{ID: id, WorkerRun: workerRun, Claim: "workflow action is evidence-backed", Action: action, Falsifier: "objective evidence disagrees"}
	if evidence.Sequence != 0 {
		decision.EvidenceThrough, decision.Evidence = evidence.Sequence, []run.EvidenceRef{evidence}
	}
	return decision
}

func workflowEvidence(t *testing.T, root, experimentID, runID string) run.EvidenceRef {
	t.Helper()
	op, err := run.Open(root, experimentID, runID)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.EvidenceForSourceLocator("workflow:" + runID + ":failure")
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func workflowPreparedRun(t *testing.T, store artifact.Store, runID string, candidate artifact.Ref, suffix string) artifact.Ref {
	t.Helper()
	put := func(value string) artifact.Ref {
		ref, err := store.Put([]byte(value + ":" + suffix))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	fixture := put("fixture")
	reset, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{Contract: experiment.FixtureResetContract, RunID: runID, Fixture: fixture, Baseline: put("baseline"), Evidence: []artifact.Ref{put("reset")}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: runID, Inputs: experiment.RunInputs{
		Harness: put("harness"), Trial: put("trial"), Candidate: candidate, Adapter: put("adapter"), OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset,
		EvidencePolicy: put("evidence-policy"), StopPolicy: put("stop-policy"), WorkerRuntime: put("worker-runtime"), Environment: put("environment"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func sealWorkflowPreparation(t *testing.T, prep *preparation.Operation, store artifact.Store) preparation.Status {
	t.Helper()
	status, err := prep.Status()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := store.Put([]byte("independent leakage assay"))
	if err != nil {
		t.Fatal(err)
	}
	if err := prep.RecordLeakageAssay(preparation.LeakageAssay{WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer", Authority: "reviewer", Method: "semantic-contrast", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{evidence}}); err != nil {
		t.Fatal(err)
	}
	basis, err := prep.ChallengeBasis()
	if err != nil {
		t.Fatal(err)
	}
	if err := prep.Challenge(preparation.Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Seal(); err != nil {
		t.Fatal(err)
	}
	status, err = prep.Status()
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func workflowCoderCompletion(t *testing.T, root, experimentID, runID string) artifact.Ref {
	t.Helper()
	op, err := run.Open(root, experimentID, runID)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		receipt, _, err := op.CoderCompletionReceipt()
		if err == nil {
			return receipt
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("workflow Coder completion did not settle")
	return artifact.Ref{}
}

func pointer[T any](value T) *T { return &value }
