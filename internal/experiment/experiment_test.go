package experiment

import (
	"errors"
	"strings"
	"testing"

	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func TestExperimentValidatesDurableFindingEvidenceAndDisposition(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "prep")
	experiment, _ := Open(root, "exp")
	if _, err := experiment.Begin("prep"); err != nil {
		t.Fatal(err)
	}
	bindTestRun(t, experiment, "run-1")
	runOperation := attachedRunWithEvidence(t, root, "exp", "run-1")
	refs := []run.EvidenceRef{
		{ExperimentID: "exp", RunID: "run-1", Sequence: 2, Item: 0},
		{ExperimentID: "exp", RunID: "run-1", Sequence: 2, Item: 1},
	}
	items := make([]run.EvidenceItem, 0, len(refs))
	for _, ref := range refs {
		item, err := runOperation.EvidenceAt(ref)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, item)
	}
	value, err := (finding.RepeatedLabelDetector{Label: "validation_failure", Threshold: 2}).Detect(items)
	if err != nil || value == nil {
		t.Fatalf("detector = %#v, %v", value, err)
	}
	if err := experiment.RecordFinding(*value); err != nil {
		t.Fatal(err)
	}
	sourceFile, _ := experiment.artifacts.Put([]byte("package owner\n\nfunc transition() {}\n"))
	current, _ := experiment.current()
	diagnosed := diagnosis.Diagnosis{
		ID: "diagnosis-1", State: diagnosis.Established, FindingIDs: []string{value.ID}, SourceSnapshot: current.begun.Source,
		SourceEvidence: []diagnosis.SourceEvidenceRef{{Artifact: sourceFile, Path: "owner.go", StartLine: 1, EndLine: 3, EstablishesOwner: true}},
		Owner:          "owner.transition", RootCause: "transition accepts duplicate retry", Invariant: "one accepted process",
		RepairBoundary: "attempt transition owner", ProhibitedPatches: []string{"ignore duplicate"},
		AcceptanceClaims: []diagnosis.Claim{{ID: "claim-1", Statement: "duplicate retry is unreachable", Falsifier: "two accepted processes"}},
	}
	if err := experiment.RecordDiagnosis(diagnosed); err != nil {
		t.Fatal(err)
	}
	outsider, _ := experiment.artifacts.Put([]byte("package outsider\nfunc transition() {}\n"))
	notInSnapshot := diagnosed
	notInSnapshot.ID = "diagnosis-outsider"
	notInSnapshot.SourceEvidence = append([]diagnosis.SourceEvidenceRef(nil), diagnosed.SourceEvidence...)
	notInSnapshot.SourceEvidence[0].Artifact = outsider
	if err := experiment.RecordDiagnosis(notInSnapshot); err == nil {
		t.Fatal("diagnosis accepted source evidence outside exact snapshot")
	}
	candidate, err := experiment.BindCandidate("candidate-1", diagnosed.ID, []byte("exact candidate bytes"))
	if err != nil || candidate.Artifact.Digest == "" {
		t.Fatalf("candidate = %#v, %v", candidate, err)
	}
	hypothesis := diagnosed
	hypothesis.ID, hypothesis.State = "diagnosis-hypothesis", diagnosis.Hypothetical
	hypothesis.SourceEvidence[0].EstablishesOwner = false
	if err := experiment.RecordDiagnosis(hypothesis); err != nil {
		t.Fatal(err)
	}
	if _, err := experiment.BindCandidate("candidate-hypothesis", hypothesis.ID, []byte("unproven")); err == nil {
		t.Fatal("hypothetical diagnosis authorized candidate")
	}
	baselineManifest, _, err := experiment.RunManifest("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := experiment.BindRun("run-2", rebindTestFixtureReset(t, experiment, "run-2", baselineManifest.RunInputs)); err != nil {
		t.Fatal(err)
	}
	candidateInputs := baselineManifest.RunInputs
	candidateInputs.Candidate = candidate.Artifact
	for _, runID := range []string{"candidate-1", "candidate-2"} {
		if _, err := experiment.BindRun(runID, rebindTestFixtureReset(t, experiment, runID, candidateInputs)); err != nil {
			t.Fatal(err)
		}
	}
	observation := comparison.Observation{
		ID: "comparison-1", CandidateID: candidate.ID,
		BaselineRuns: []string{"run-1", "run-2"}, CandidateRuns: []string{"candidate-1", "candidate-2"},
		Policy:        comparison.Policy{MinimumRepetitions: 2, RequiredClaims: []string{"claim-1"}},
		ClaimDeltas:   []comparison.ClaimDelta{{ClaimID: "claim-1", BaselineFailures: 2, CandidateFailures: 0, HeldOut: true}},
		ValidityFacts: []comparison.ValidityFact{{Kind: "environment", Valid: true, Detail: "same controlled fixture"}},
	}
	compared, err := experiment.Compare(observation)
	if err != nil || compared.Verdict != comparison.SupportedImprovement {
		t.Fatalf("comparison = %#v, %v", compared, err)
	}
	unowned := observation
	unowned.ID = "comparison-unowned"
	unowned.Policy.RequiredClaims = []string{"claim-not-in-diagnosis"}
	if _, err := experiment.Compare(unowned); err == nil {
		t.Fatal("comparison accepted claim not owned by candidate diagnosis")
	}
	single := observation
	single.ID, single.BaselineRuns, single.CandidateRuns = "comparison-single", []string{"run-1"}, []string{"candidate-1"}
	if compared, err := experiment.Compare(single); err != nil || compared.Verdict != comparison.Inconclusive {
		t.Fatalf("single comparison = %#v, %v", compared, err)
	}
	gateItem := gate.Item{
		ID: "claim-1", Status: gate.Passed, Statement: "duplicate retry is unreachable", Impact: "one process is accepted",
		Evidence: refs, Severity: finding.SeverityHigh, Confidence: finding.ConfidenceHigh, Falsifier: "two accepted processes",
	}
	passed, err := experiment.RecordGate(gate.Spec{ID: "gate-pass", CandidateID: candidate.ID, ComparisonID: observation.ID, Items: []gate.Item{gateItem}})
	if err != nil || passed.Verdict != gate.Pass || passed.Receipt.Candidate != candidate.Artifact {
		t.Fatalf("passing gate = %#v, %v", passed, err)
	}
	changed, err := experiment.BindCandidate("candidate-2", diagnosed.ID, []byte("changed candidate bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := experiment.RecordGate(gate.Spec{ID: "gate-stale", CandidateID: changed.ID, ComparisonID: observation.ID, Items: []gate.Item{gateItem}}); err == nil {
		t.Fatal("changed candidate reused prior comparison gate")
	}
	gateItem.ID, gateItem.Status, gateItem.Statement = "verification", gate.Blocked, "verification is blocked"
	blocked, err := experiment.RecordGate(gate.Spec{ID: "gate-blocked", CandidateID: changed.ID, Items: []gate.Item{gateItem}})
	if err != nil || blocked.Verdict != gate.Block {
		t.Fatalf("blocked gate = %#v, %v", blocked, err)
	}
	if blocker, _, err := experiment.Finding("gate-blocked.verification"); err != nil || blocker.Class != "gate_blocker" {
		t.Fatalf("gate blocker finding = %#v, %v", blocker, err)
	}
	handoff, err := experiment.RenderHandoff([]string{value.ID})
	if err != nil {
		t.Fatal(err)
	}
	document, _ := experiment.artifacts.Read(handoff.Artifact)
	if handoff.EvidenceCount != 2 || strings.Contains(string(document), "first") || strings.Contains(string(document), "second") || strings.Contains(string(document), "RootCause") || !strings.Contains(string(document), "sha256:") {
		t.Fatalf("handoff leaked or omitted evidence boundary: %s", document)
	}
	disposition := finding.Disposition{FindingID: value.ID, Kind: finding.ExperimentRequired, Authority: "reviewer", Reason: "repeat under held-out input"}
	if err := experiment.RecordDisposition(disposition); err != nil {
		t.Fatal(err)
	}
	status, err := experiment.Status()
	if err != nil || len(status.FindingIDs) != 2 || len(status.DiagnosisIDs) != 2 || len(status.CandidateIDs) != 2 || len(status.RunIDs) != 4 || len(status.ComparisonIDs) != 2 || len(status.GateIDs) != 2 || status.DispositionCount != 1 {
		t.Fatalf("status = %#v, %v", status, err)
	}
	reopened, _ := Open(root, "exp")
	got, gotDisposition, err := reopened.Finding(value.ID)
	if err != nil || got.ID != value.ID || gotDisposition == nil || gotDisposition.Kind != finding.ExperimentRequired {
		t.Fatalf("replayed finding = %#v %#v, %v", got, gotDisposition, err)
	}
}

func TestFindingRejectsMissingOrCrossExperimentEvidence(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "prep")
	experiment, _ := Open(root, "exp")
	_, _ = experiment.Begin("prep")
	base := finding.Finding{
		ID: "finding-1", Class: "failure", Severity: finding.SeverityHigh,
		Symptom: "failed", Impact: "task incomplete", Confidence: finding.ConfidenceHigh, Falsifier: "show success",
	}
	base.Evidence = []run.EvidenceRef{{ExperimentID: "other", RunID: "run", Sequence: 1}}
	if err := experiment.RecordFinding(base); err == nil {
		t.Fatal("cross-experiment evidence was accepted")
	}
	base.Evidence = []run.EvidenceRef{{ExperimentID: "exp", RunID: "missing", Sequence: 1}}
	if err := experiment.RecordFinding(base); err == nil {
		t.Fatal("missing evidence was accepted")
	}
	if err := experiment.RecordDisposition(finding.Disposition{FindingID: "missing", Kind: finding.AcceptedRisk, Authority: "agent", Reason: "ignore"}); err == nil {
		t.Fatal("invalid risk disposition was accepted")
	}
}

func TestExperimentRequiresSealedPreparation(t *testing.T) {
	root := t.TempDir()
	prep, _ := preparation.Open(root, "open-prep")
	_, _ = prep.Begin(preparation.BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "source.txt", Content: []byte("source")}}, Authority: "designer"})
	experiment, _ := Open(root, "exp")
	if _, err := experiment.Begin("open-prep"); !errors.Is(err, ErrPreparationNotSealed) {
		t.Fatalf("unsealed preparation error = %v", err)
	}
}
