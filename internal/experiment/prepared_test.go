package experiment

import (
	"testing"

	"github.com/yansircc/agentlab/internal/run"
)

func TestPreparedRunBindsOnlyItsHostIssuedInputs(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "prepared-prep")
	op, _ := Open(root, "prepared-exp")
	_, _ = op.Begin("prepared-prep")
	_, evidence, _, _ := spliceParent(t, op, root, "parent")
	inputs := testRunInputs(t, op, "child", "prepared")
	prepared, err := RecordPreparedRun(op.artifacts, PreparedRun{Contract: PreparedRunContract, RunID: "child", Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	decision := SupervisorDecision{ID: "bind-child", WorkerRun: "parent", EvidenceThrough: evidence.Sequence, Claim: "child inputs are Host-prepared", Action: DecisionRunBinding, Evidence: []run.EvidenceRef{evidence}, Falsifier: "manifest differs from prepared input"}
	if _, err := op.BindPreparedRunWithDecision(DecisionBoundRunBinding{Decision: decision, RunID: "child"}, NewFreshOrigin(), prepared); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := op.RunManifest("child")
	if err != nil || manifest.RunInputs != inputs {
		t.Fatalf("prepared manifest = %#v, %v", manifest, err)
	}
	if _, err := op.BindPreparedRunWithDecision(DecisionBoundRunBinding{Decision: SupervisorDecision{ID: "wrong-run", WorkerRun: "parent", EvidenceThrough: evidence.Sequence, Claim: "wrong run", Action: DecisionRunBinding, Evidence: []run.EvidenceRef{evidence}, Falsifier: "mismatch"}, RunID: "other"}, NewFreshOrigin(), prepared); err == nil {
		t.Fatal("prepared inputs bound a different run id")
	}
}
