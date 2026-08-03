package experiment

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
)

func TestSpliceOriginDerivesVerifiedLineage(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "origin-prep")
	op, _ := Open(root, "origin-exp")
	_, _ = op.Begin("origin-prep")
	_, evidence, checkpoint, prefix := spliceParent(t, op, root, "parent")
	intervention := putTestArtifact(t, op, "reobserve public contract")
	origin := testSpliceOrigin(t, "parent", evidence, checkpoint, prefix, &intervention)
	if _, err := op.BindRun("child", origin, testRunInputs(t, op, "child", "child")); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := op.RunManifest("child")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := manifest.Origin.Splice()
	if !ok || got.ParentRun != "parent" || got.RuntimeCheckpoint != checkpoint || got.PublicPrefix != prefix || got.Intervention == nil || *got.Intervention != intervention {
		t.Fatalf("derived splice origin = %#v, %t", got, ok)
	}
	lineage, err := op.Lineage()
	if err != nil {
		t.Fatal(err)
	}
	if len(lineage.Roots) != 1 || lineage.Roots[0] != "parent" || len(lineage.Edges) != 1 || lineage.Edges[0].ParentRun != "parent" || lineage.Edges[0].ChildRun != "child" || lineage.Edges[0].Intervention == nil || *lineage.Edges[0].Intervention != intervention {
		t.Fatalf("replayed lineage = %#v", lineage)
	}
}

func TestRunOriginRejectsIllegalProducts(t *testing.T) {
	cases := []string{
		`{"kind":"fresh","parent_run":"parent"}`,
		`{"kind":"fresh","intervention":null}`,
		`{"kind":"splice","parent_run":"parent"}`,
		`{"kind":"unknown"}`,
	}
	for _, input := range cases {
		var origin RunOrigin
		if err := json.Unmarshal([]byte(input), &origin); err == nil {
			t.Fatalf("illegal origin accepted: %s", input)
		}
	}
}

func TestSpliceOriginRejectsUnownedFacts(t *testing.T) {
	root := t.TempDir()
	sealPreparation(t, root, "unowned-prep")
	op, _ := Open(root, "unowned-exp")
	_, _ = op.Begin("unowned-prep")
	_, evidence, checkpoint, prefix := spliceParent(t, op, root, "parent")
	_, _, foreignCheckpoint, _ := spliceParent(t, op, root, "other")
	fake := artifact.Ref{Algorithm: "sha256", Digest: strings.Repeat("a", 64), Size: 1}
	intervention := fake
	cases := []struct {
		name   string
		runID  string
		origin RunOrigin
	}{
		{"missing parent", "missing-parent", testSpliceOrigin(t, "missing", withRun(evidence, "missing"), checkpoint, prefix, nil)},
		{"self parent", "self", testSpliceOrigin(t, "self", withRun(evidence, "self"), checkpoint, prefix, nil)},
		{"cross experiment evidence", "cross", testSpliceOrigin(t, "parent", withExperiment(evidence, "other"), checkpoint, prefix, nil)},
		{"foreign checkpoint", "foreign", testSpliceOrigin(t, "parent", evidence, foreignCheckpoint, prefix, nil)},
		{"missing public prefix", "missing-prefix", testSpliceOrigin(t, "parent", evidence, checkpoint, fake, nil)},
		{"missing intervention", "missing-intervention", testSpliceOrigin(t, "parent", evidence, checkpoint, prefix, &intervention)},
	}
	for _, test := range cases {
		if _, err := op.BindRun(test.runID, test.origin, testRunInputs(t, op, test.runID, test.name)); err == nil {
			t.Fatalf("%s was accepted", test.name)
		}
	}
}

func TestRunOriginCycleIsRejected(t *testing.T) {
	if err := validateAcyclicOrigins([]string{"a", "b"}, map[string]string{"a": "b", "b": "a"}); err == nil {
		t.Fatal("origin cycle was accepted")
	}
}

func spliceParent(t *testing.T, op *Operation, root, runID string) (*run.Operation, run.EvidenceRef, artifact.Ref, artifact.Ref) {
	t.Helper()
	bindTestRun(t, op, runID)
	parent := attachedRunWithEvidence(t, root, op.id, runID)
	checkpoint, err := parent.RecordRuntimeCheckpoint(run.RuntimeCheckpointSpec{
		Adapter: "test", Session: []byte(runID + "-session"), OpaqueState: []byte(runID + "-opaque"), PublicPrefix: []byte(runID + "-public-prefix"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return parent, run.EvidenceRef{ExperimentID: op.id, RunID: runID, Sequence: 2, Item: 0}, checkpoint.Checkpoint, checkpoint.PublicPrefix
}

func testSpliceOrigin(t *testing.T, parent string, evidence run.EvidenceRef, checkpoint, prefix artifact.Ref, intervention *artifact.Ref) RunOrigin {
	t.Helper()
	origin, err := NewSpliceOrigin(SpliceOriginSpec{
		ParentRun: parent, ParentEvidence: evidence, RuntimeCheckpoint: checkpoint, PublicPrefix: prefix,
		Intervention: intervention, ReasonEvidence: []run.EvidenceRef{evidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	return origin
}

func withRun(value run.EvidenceRef, runID string) run.EvidenceRef {
	value.RunID = runID
	return value
}

func withExperiment(value run.EvidenceRef, experimentID string) run.EvidenceRef {
	value.ExperimentID = experimentID
	return value
}
