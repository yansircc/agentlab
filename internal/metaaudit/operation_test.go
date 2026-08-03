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
	evaluated, runOp, evidence := auditFixture(t)
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
	finding := Finding{ID: "late-stop", DecisionID: "stop", WorkerRun: "worker", EvidenceThrough: evidence.Sequence, WorkerEvidence: []run.EvidenceRef{evidence}, Claim: "stop followed a material failure", Falsifier: "a later decision cites only future evidence", GroundTruth: groundTruth}
	if err := audit.Record(evaluated, finding); err != nil {
		t.Fatal(err)
	}
	if err := appendEvidence(runOp, evidence.Sequence+1); err != nil {
		t.Fatal(err)
	}
	finding.ID, finding.WorkerEvidence[0].Sequence = "hindsight", evidence.Sequence+1
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit accepted future evidence")
	}
	if err := audit.Seal(); err != nil {
		t.Fatal(err)
	}
	finding.ID = "after-seal"
	if err := audit.Record(evaluated, finding); err == nil {
		t.Fatal("meta-audit accepted a post-seal finding")
	}
}

func auditFixture(t *testing.T) (string, *run.Operation, run.EvidenceRef) {
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
	if _, err := experimentOp.BindRun("worker", experiment.NewFreshOrigin(), inputs); err != nil {
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
	return root, runOp, ref
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
	return writer.Commit([]byte(strconv.FormatUint(sequence, 10)), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: run.EvidenceToolResult, Raw: []byte("oracle failure"), Label: "target_mismatch"}}})
}
