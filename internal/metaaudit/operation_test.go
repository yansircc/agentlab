package metaaudit

import (
	"strconv"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func TestMetaAuditRejectsFutureEvidenceAndSealsFindings(t *testing.T) {
	evaluated, _, evidence, oracleEvidence := auditFixture(t)
	auditRoot := t.TempDir()
	audit, err := Open(auditRoot, "trial")
	if err != nil {
		t.Fatal(err)
	}
	groundTruth, err := audit.artifacts.Put([]byte("target mismatch is material"))
	if err != nil {
		t.Fatal(err)
	}
	trial := Trial{Contract: Contract, ExperimentID: "experiment", EvaluatedScope: artifact.NewStore(evaluated + "/artifacts").Scope(), GroundTruth: groundTruth}
	if err := audit.Begin(trial); err != nil {
		t.Fatal(err)
	}
	finding := Finding{ID: "late-stop", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: evidence.Sequence, WorkerEvidence: []run.EvidenceRef{evidence}, OracleEvidence: []run.EvidenceRef{oracleEvidence}, Claim: "stop followed a material failure", Falsifier: "a later decision cites only future evidence", GroundTruth: groundTruth}
	if err := audit.Record(evaluated, finding); err != nil {
		t.Fatal(err)
	}
	review := Review{ID: "duplicate-assessment", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: evidence.Sequence, WorkerEvidence: []run.EvidenceRef{evidence}, Claim: "the stop was reviewed", Falsifier: "the decision lacks its cited public evidence", GroundTruth: groundTruth}
	if err := audit.RecordReview(evaluated, review); err == nil {
		t.Fatal("meta-audit accepted a second assessment for one decision")
	}
	if err := audit.Seal(); err != nil {
		t.Fatal(err)
	}
	finding.ID = "after-seal"
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit accepted a post-seal finding")
	}
}

func TestMetaAuditRejectsFutureObjectiveOracleEvidence(t *testing.T) {
	evaluated, runOp, workerEvidence, oracleEvidence := auditFixture(t)
	if err := appendEvidence(runOp, workerEvidence.Sequence+1); err != nil {
		t.Fatal(err)
	}
	auditRoot := t.TempDir()
	audit, err := Open(auditRoot, "trial")
	if err != nil {
		t.Fatal(err)
	}
	groundTruth, err := audit.artifacts.Put([]byte("target mismatch is material"))
	if err != nil {
		t.Fatal(err)
	}
	trial := Trial{Contract: Contract, ExperimentID: "experiment", EvaluatedScope: artifact.NewStore(evaluated + "/artifacts").Scope(), GroundTruth: groundTruth}
	if err := audit.Begin(trial); err != nil {
		t.Fatal(err)
	}
	oracleEvidence.Sequence++
	finding := Finding{ID: "future-oracle", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: 2, WorkerEvidence: []run.EvidenceRef{workerEvidence}, OracleEvidence: []run.EvidenceRef{oracleEvidence}, Claim: "a material failure was missed", Falsifier: "all cited evidence predates the decision", GroundTruth: groundTruth}
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit accepted objective oracle evidence after the decision prefix")
	}
}

func TestMetaAuditReviewCoversNoFindingDecision(t *testing.T) {
	evaluated, _, evidence, oracleEvidence := auditFixture(t)
	auditRoot := t.TempDir()
	audit, err := Open(auditRoot, "trial")
	if err != nil {
		t.Fatal(err)
	}
	groundTruth, err := audit.artifacts.Put([]byte("target mismatch is material"))
	if err != nil {
		t.Fatal(err)
	}
	trial := Trial{Contract: Contract, ExperimentID: "experiment", EvaluatedScope: artifact.NewStore(evaluated + "/artifacts").Scope(), GroundTruth: groundTruth}
	if err := audit.Begin(trial); err != nil {
		t.Fatal(err)
	}
	review := Review{ID: "stop-reviewed", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: evidence.Sequence, WorkerEvidence: []run.EvidenceRef{evidence}, Claim: "the stop was supported by the cited prefix", Falsifier: "a cited event is absent or newer than the decision", GroundTruth: groundTruth}
	if err := audit.RecordReview(evaluated, review); err != nil {
		t.Fatal(err)
	}
	status, err := audit.Status()
	if err != nil || len(status.ReviewIDs) != 1 || status.ReviewIDs[0] != review.ID || len(status.FindingIDs) != 0 {
		t.Fatalf("review status = %#v, %v", status, err)
	}
	finding := Finding{ID: "duplicate-finding", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: evidence.Sequence, WorkerEvidence: []run.EvidenceRef{evidence}, OracleEvidence: []run.EvidenceRef{oracleEvidence}, Claim: "the stop was wrong", Falsifier: "the decision was sound", GroundTruth: groundTruth}
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit accepted a finding after its no-finding review")
	}
	if err := audit.Seal(); err != nil {
		t.Fatal(err)
	}
}

func TestMetaAuditFindingRequiresIndependentObjectiveOracleEvidence(t *testing.T) {
	evaluated, _, workerEvidence, oracleEvidence := auditFixture(t)
	auditRoot := t.TempDir()
	audit, err := Open(auditRoot, "trial")
	if err != nil {
		t.Fatal(err)
	}
	groundTruth, err := audit.artifacts.Put([]byte("target mismatch is material"))
	if err != nil {
		t.Fatal(err)
	}
	trial := Trial{Contract: Contract, ExperimentID: "experiment", EvaluatedScope: artifact.NewStore(evaluated + "/artifacts").Scope(), GroundTruth: groundTruth}
	if err := audit.Begin(trial); err != nil {
		t.Fatal(err)
	}
	finding := Finding{ID: "missing-oracle", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: workerEvidence.Sequence, WorkerEvidence: []run.EvidenceRef{workerEvidence}, Claim: "stop missed a material failure", Falsifier: "the objective oracle was unavailable", GroundTruth: groundTruth}
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit finding omitted objective oracle evidence")
	}
	finding.ID, finding.OracleEvidence = "reused-worker-evidence", []run.EvidenceRef{workerEvidence}
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit finding reused Worker evidence as its objective oracle")
	}
	finding.ID, finding.OracleEvidence = "wrong-oracle-kind", []run.EvidenceRef{oracleEvidence}
	wrong := oracleEvidence
	wrong.Item = 2
	finding.OracleEvidence = []run.EvidenceRef{wrong}
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit finding accepted a non-oracle event")
	}
	finding.ID, finding.OracleEvidence = "objective-oracle", []run.EvidenceRef{oracleEvidence}
	if err := audit.Record(evaluated, finding); err != nil {
		t.Fatal(err)
	}
}

func TestMetaAuditCoverageRequiresEveryDecision(t *testing.T) {
	auditState := state{
		findings: map[string]Finding{"finding": {ID: "finding", DecisionID: "stop"}},
		reviews:  map[string]Review{"review": {ID: "review", DecisionID: "gate"}},
	}
	if !auditState.covers([]string{"stop", "gate"}) {
		t.Fatal("finding and review did not cover their decisions")
	}
	if auditState.covers([]string{"stop", "checkpoint"}) {
		t.Fatal("missing decision assessment was accepted")
	}
	if auditState.clean() {
		t.Fatal("adverse meta finding was treated as a clean audit")
	}
	auditState = state{reviews: map[string]Review{"review": {ID: "review", DecisionID: "gate"}}}
	if !auditState.clean() {
		t.Fatal("no-finding review was treated as an adverse finding")
	}
}

func auditFixture(t *testing.T) (string, *run.Operation, run.EvidenceRef, run.EvidenceRef) {
	t.Helper()
	root := t.TempDir()
	store := artifact.NewStore(root + "/artifacts")
	prep, _ := preparation.Open(root, "prep")
	if _, err := prep.Begin(preparation.BeginSpec{UserIntent: []byte("deploy release"), SourceFiles: []source.InputFile{{Path: "main.go", Content: []byte("package main")}}, Authority: "human"}); err != nil {
		t.Fatal(err)
	}
	prepared, _ := prep.Status()
	assay, _ := store.Put([]byte("independent review"))
	if err := prep.RecordLeakageAssay(preparation.LeakageAssay{WorkerInput: prepared.WorkerInput, SourceSnapshot: prepared.Source, Reviewer: "reviewer", Authority: "reviewer", Method: "contrast", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{assay}}); err != nil {
		t.Fatal(err)
	}
	basis, _ := prep.ChallengeBasis()
	if err := prep.Challenge(preparation.Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Seal(); err != nil {
		t.Fatal(err)
	}
	experimentOp, _ := experiment.Open(root, "experiment")
	if _, err := experimentOp.Begin("prep"); err != nil {
		t.Fatal(err)
	}
	put := func(value string) artifact.Ref {
		ref, err := store.Put([]byte(value))
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
	reset, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{Contract: experiment.FixtureResetContract, RunID: "worker", Fixture: fixture, Baseline: put("baseline"), Evidence: []artifact.Ref{put("reset")}})
	if err != nil {
		t.Fatal(err)
	}
	inputs := experiment.RunInputs{Harness: put("harness"), Trial: put("trial"), Candidate: candidate, Adapter: put("adapter"), OracleSet: put("oracles"), Fixture: fixture, FixtureReset: reset, EvidencePolicy: put("evidence"), StopPolicy: put("policy"), WorkerRuntime: put("runtime"), Environment: put("environment")}
	preparedRef, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: "worker", Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := experimentOp.BindPreparedRun("worker", experiment.NewFreshOrigin(), preparedRef); err != nil {
		t.Fatal(err)
	}
	runOp, _ := run.Open(root, "experiment", "worker")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := runOp.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "worker", InitialCursor: []byte("0"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	if err := appendEvidence(runOp, 2); err != nil {
		t.Fatal(err)
	}
	stop, _ := run.EncodeStopPayload(run.StopPayload{Reason: "material failure"})
	payload, _ := store.Put(stop)
	ref := run.EvidenceRef{ExperimentID: "experiment", RunID: "worker", Sequence: 2, Item: 0}
	decision := experiment.SupervisorDecision{ID: "stop", WorkerRun: "worker", EvidenceThrough: 2, Claim: "target mismatch", Action: experiment.DecisionStop, Evidence: []run.EvidenceRef{ref}, Falsifier: "target status matches receipt"}
	if err := experimentOp.CommitDecisionBoundEffect(experiment.DecisionBoundEffect{Decision: decision, Intent: effect.Intent{ID: "stop", RunID: "worker", Kind: effect.Stop, Payload: payload}}); err != nil {
		t.Fatal(err)
	}
	return root, runOp, ref, run.EvidenceRef{ExperimentID: "experiment", RunID: "worker", Sequence: 2, Item: 1}
}

func appendEvidence(operation *run.Operation, sequence uint64) (resultErr error) {
	writer, _, err := operation.AcquireAdapterWriter("test")
	if err != nil {
		return err
	}
	defer func() {
		if err := writer.Close(); resultErr == nil {
			resultErr = err
		}
	}()
	return writer.Commit([]byte(strconv.FormatUint(sequence, 10)), run.AdapterBatch{Events: []run.AdapterEvent{
		{Kind: run.EvidenceToolResult, Raw: []byte("Worker observed target mismatch"), Label: "target_mismatch"},
		{Kind: run.EvidenceOracle, Raw: []byte("objective oracle: target mismatch"), Label: "objective_oracle"},
		{Kind: run.EvidenceToolResult, Raw: []byte("other public evidence"), Label: "worker_observation"},
	}})
}
