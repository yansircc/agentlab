package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func TestExperimentReviewAndHandoffCLI(t *testing.T) {
	root := t.TempDir()
	prep, _ := preparation.Open(root, "review-prep")
	_, _ = prep.Begin(preparation.BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "owner.go", Content: []byte("package owner\nfunc transition() {}\n")}}, Authority: "designer"})
	recordTestLeakageAssay(t, root, prep)
	basis, _ := prep.ChallengeBasis()
	_ = prep.Challenge(preparation.Challenge{Basis: basis})
	_, _ = prep.Seal()
	begin, err := dispatch([]string{"experiment", "begin", "-root", root, "-experiment", "review-exp", "-preparation", "review-prep"})
	if err != nil || begin.(experiment.Status).PreparationID != "review-prep" {
		t.Fatalf("experiment begin = %#v, %v", begin, err)
	}
	experimentOperation, _ := experiment.Open(root, "review-exp")
	bindExistingExperimentRun(t, root, experimentOperation, "review-run")
	runOperation, _ := run.Open(root, "review-exp", "review-run")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	_, _ = runOperation.BeginAttached(run.AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor-0"), Policy: policy, Capabilities: run.RequiredAdapterCapabilities()})
	writer, _, _ := runOperation.AcquireAdapterWriter("test")
	_ = writer.Commit([]byte("cursor-1"), run.AdapterBatch{Events: []run.AdapterEvent{
		{Kind: "tool_result", Label: "retry", Raw: []byte("one")},
		{Kind: "tool_result", Label: "retry", Raw: []byte("two")},
		{Kind: "tool_call", Label: "private_shell", Raw: []byte(`{"command":"bypass"}`)},
	}})
	_ = writer.Close()
	files := t.TempDir()
	detectRequest := writeJSONFile(t, files, "detect.json", map[string]any{
		"experiment_id": "review-exp", "label": "retry", "threshold": 2,
		"evidence": []run.EvidenceRef{
			{ExperimentID: "review-exp", RunID: "review-run", Sequence: 2, Item: 0},
			{ExperimentID: "review-exp", RunID: "review-run", Sequence: 2, Item: 1},
		},
	})
	detected, err := dispatch([]string{"review", "detect-repeated", "-root", root, "-request", detectRequest})
	if err != nil {
		t.Fatal(err)
	}
	value := detected.(*finding.Finding)
	bypassRequest := writeJSONFile(t, files, "bypass.json", map[string]any{
		"experiment_id": "review-exp", "allowed_tools": []string{"read"},
		"evidence": []run.EvidenceRef{{ExperimentID: "review-exp", RunID: "review-run", Sequence: 2, Item: 2}},
	})
	bypass, err := dispatch([]string{"review", "detect-bypass", "-root", root, "-request", bypassRequest})
	if err != nil || bypass.(*finding.Finding).Class != "public_contract_bypass" {
		t.Fatalf("public bypass = %#v, %v", bypass, err)
	}
	store := artifact.NewStore(root + "/artifacts")
	sourceFile, _ := store.Put([]byte("package owner\nfunc transition() {}\n"))
	prepStatus, _ := prep.Status()
	diagnosisValue := diagnosis.Diagnosis{
		ID: "cli-diagnosis", State: diagnosis.Established, FindingIDs: []string{value.ID}, SourceSnapshot: prepStatus.Source,
		SourceEvidence: []diagnosis.SourceEvidenceRef{{Artifact: sourceFile, Path: "owner.go", StartLine: 1, EndLine: 2, EstablishesOwner: true}},
		Owner:          "owner.transition", RootCause: "retry transition duplicates work", Invariant: "one accepted transition",
		RepairBoundary: "transition owner", AcceptanceClaims: []diagnosis.Claim{{ID: "cli-claim", Statement: "duplicate is unreachable", Falsifier: "two accepted transitions"}},
	}
	diagnosisRequest := writeJSONFile(t, files, "diagnosis.json", map[string]any{"experiment_id": "review-exp", "diagnosis": diagnosisValue})
	if _, err := dispatch([]string{"diagnose", "record", "-root", root, "-request", diagnosisRequest}); err != nil {
		t.Fatal(err)
	}
	handoff, err := experimentOperation.RenderHandoffWithDecision(experiment.SupervisorDecision{
		ID: "cli-coder-handoff", WorkerRun: "review-run", EvidenceThrough: 2, Claim: "retry evidence is bounded", Action: experiment.DecisionHandoff,
		Evidence: []run.EvidenceRef{{ExperimentID: "review-exp", RunID: "review-run", Sequence: 2, Item: 0}}, Falsifier: "handoff cites future evidence",
	}, []string{value.ID})
	if err != nil {
		t.Fatal(err)
	}
	candidateRef, err := source.Build(store, []source.InputFile{{Path: "owner.go", Content: []byte("package owner\nfunc transition() {}\n")}})
	if err != nil {
		t.Fatal(err)
	}
	completion := completeExistingCandidate(t, root, experimentOperation, "cli-coder", handoff.Artifact, prepStatus.Source, candidateRef, experiment.SupervisorDecision{
		ID: "cli-coder-start", WorkerRun: "review-run", EvidenceThrough: 2, Claim: "Coder receives evidence-only handoff", Action: experiment.DecisionCoderStart,
		Evidence: []run.EvidenceRef{{ExperimentID: "review-exp", RunID: "review-run", Sequence: 2, Item: 0}}, Falsifier: "Coder start lacks exact handoff",
	})
	candidateRequest := writeJSONFile(t, files, "candidate.json", map[string]any{
		"experiment_id": "review-exp", "candidate_id": "cli-candidate", "diagnosis_id": diagnosisValue.ID, "coder_run_id": "cli-coder", "completion": completion,
	})
	candidateResult, err := dispatch([]string{"diagnose", "bind-candidate", "-root", root, "-request", candidateRequest})
	if err != nil {
		t.Fatal(err)
	}
	candidate := candidateResult.(diagnosis.RepairCandidate)
	baselineManifest, _, _ := experimentOperation.RunManifest("review-run")
	bindManifestCLI := func(runID string, inputs experiment.RunInputs) {
		proofBytes, err := store.Read(inputs.FixtureReset)
		if err != nil {
			t.Fatal(err)
		}
		var previous experiment.FixtureResetProof
		if err := json.Unmarshal(proofBytes, &previous); err != nil {
			t.Fatal(err)
		}
		evidence, _ := store.Put([]byte("reset-evidence:" + runID))
		inputs.FixtureReset, err = experiment.RecordFixtureReset(store, experiment.FixtureResetProof{
			Contract: experiment.FixtureResetContract, RunID: runID, Fixture: inputs.Fixture,
			Baseline: previous.Baseline, Evidence: []artifact.Ref{evidence},
		})
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: runID, Inputs: inputs})
		if err != nil {
			t.Fatal(err)
		}
		request := writeJSONFile(t, files, runID+".json", map[string]any{"experiment_id": "review-exp", "run_id": runID, "origin": experiment.NewFreshOrigin(), "prepared": prepared})
		if _, err := dispatch([]string{"experiment", "bind-run", "-root", root, "-request", request}); err != nil {
			t.Fatal(err)
		}
	}
	bindManifestCLI("baseline-2", baselineManifest.RunInputs)
	candidateInputs := baselineManifest.RunInputs
	candidateInputs.Candidate = candidate.Artifact
	bindManifestCLI("candidate-1", candidateInputs)
	bindManifestCLI("candidate-2", candidateInputs)
	comparisonValue := comparison.Observation{
		ID: "cli-comparison", CandidateID: candidate.ID,
		BaselineRuns: []string{"review-run", "baseline-2"}, CandidateRuns: []string{"candidate-1", "candidate-2"},
		Policy:        comparison.Policy{MinimumRepetitions: 2, RequiredClaims: []string{"cli-claim"}},
		ClaimDeltas:   []comparison.ClaimDelta{{ClaimID: "cli-claim", BaselineFailures: 2, CandidateFailures: 0}},
		ValidityFacts: []comparison.ValidityFact{{Kind: "environment", Valid: true, Detail: "equivalent"}},
	}
	comparisonRequest := writeJSONFile(t, files, "comparison.json", map[string]any{"experiment_id": "review-exp", "observation": comparisonValue})
	compared, err := dispatch([]string{"compare", "record", "-root", root, "-request", comparisonRequest})
	if err != nil || compared.(comparison.Result).Verdict != comparison.SupportedImprovement {
		t.Fatalf("comparison = %#v, %v", compared, err)
	}
	gateRequest := writeJSONFile(t, files, "gate.json", map[string]any{
		"experiment_id": "review-exp",
		"gate": gate.Spec{ID: "cli-gate", CandidateID: candidate.ID, ComparisonID: comparisonValue.ID, Items: []gate.Item{{
			ID: "cli-claim", Status: gate.Passed, Statement: "duplicate is unreachable", Impact: "single accepted transition",
			Evidence: value.Evidence, Severity: finding.SeverityHigh, Confidence: finding.ConfidenceHigh, Falsifier: "two accepted transitions",
		}}},
	})
	gated, err := dispatch([]string{"gate", "record", "-root", root, "-request", gateRequest})
	if err != nil || gated.(gate.Result).Verdict != gate.Pass || gated.(gate.Result).Receipt.Candidate != candidate.Artifact {
		t.Fatalf("gate = %#v, %v", gated, err)
	}
	shown, err := dispatch([]string{"gate", "show", "-root", root, "-experiment", "review-exp", "-gate", "cli-gate"})
	if err != nil || shown.(gate.Result).Receipt.Candidate != candidate.Artifact {
		t.Fatalf("gate show = %#v, %v", shown, err)
	}
	handoffRequest := writeJSONFile(t, files, "handoff.json", map[string]any{"experiment_id": "review-exp", "finding_ids": []string{value.ID}})
	handoffResult, err := dispatch([]string{"review", "handoff", "-root", root, "-request", handoffRequest})
	if err != nil || handoffResult.(experiment.HandoffResult).EvidenceCount != 2 {
		t.Fatalf("handoff = %#v, %v", handoffResult, err)
	}
	page, err := dispatch([]string{"inspect", "-root", root, "-experiment", "review-exp", "-after", "0", "-limit", "20"})
	records, ok := page.([]ledger.Record)
	if err != nil || !ok || len(records) < 8 || records[2].Kind != "finding_recorded" {
		t.Fatalf("experiment inspect = %#v, %v", page, err)
	}
}
