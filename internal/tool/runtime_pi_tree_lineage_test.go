package tool

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
)

func TestPiRuntimeTreeRetainsParentEvidenceForSplicedPublicPrefix(t *testing.T) {
	root := t.TempDir()
	binding := Binding{Root: root, ExperimentID: "exp"}
	_ = renderOwnedHandoff(t, binding)
	parent := testWorkerLaunch(t)
	child := testWorkerLaunch(t)
	for _, launch := range []*PiWorkerLaunch{parent, child} {
		if err := os.MkdirAll(launch.Launch.RuntimeRoot, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	parentSession := filepath.Join(parent.Launch.RuntimeRoot, "session.jsonl")
	childSession := filepath.Join(child.Launch.RuntimeRoot, "session.jsonl")
	parentTree := []string{
		`{"type":"session","version":3,"id":"parent-session"}`,
		`{"type":"message","id":"inherited-user","parentId":null,"message":{"role":"user","content":"public"}}`,
	}
	childTree := append([]string{}, parentTree...)
	childTree[0] = `{"type":"session","version":3,"id":"child-session"}`
	writePiTree(t, parentSession, parentTree)
	writePiTree(t, childSession, childTree)
	parsed, err := piadapter.ReadPublicTree(parentSession)
	if err != nil {
		t.Fatal(err)
	}
	source, err := parsed.Entries[0].EvidenceSource()
	if err != nil {
		t.Fatal(err)
	}
	parentRun, err := run.Open(root, "exp", "worker")
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := parentRun.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit([]byte("prefix-cursor"), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: run.EvidenceUserMessage, SourceLocator: source, Label: "user_message", Raw: []byte(`"public"`)}}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	parentEvidence, err := parentRun.EvidenceForSourceLocator(source)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := parentRun.RecordRuntimeCheckpoint(run.RuntimeCheckpointSpec{Adapter: "test", Session: []byte("parent-session"), OpaqueState: []byte("opaque"), PublicPrefix: []byte("public-prefix")})
	if err != nil {
		t.Fatal(err)
	}
	experimentOp, err := experiment.Open(root, "exp")
	if err != nil {
		t.Fatal(err)
	}
	parentManifest, _, err := experimentOp.RunManifest("worker")
	if err != nil {
		t.Fatal(err)
	}
	store := artifact.NewStore(filepath.Join(root, "artifacts"))
	put := func(value string) artifact.Ref {
		ref, err := store.Put([]byte(value))
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	inputs := parentManifest.RunInputs
	inputs.Fixture = put("child-fixture")
	reset, err := experiment.RecordFixtureReset(store, experiment.FixtureResetProof{Contract: experiment.FixtureResetContract, RunID: "child", Fixture: inputs.Fixture, Baseline: put("child-baseline"), Evidence: []artifact.Ref{put("child-reset")}})
	if err != nil {
		t.Fatal(err)
	}
	inputs.FixtureReset = reset
	prepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: "child", Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	origin, err := experiment.NewSpliceOrigin(experiment.SpliceOriginSpec{ParentRun: "worker", ParentEvidence: parentEvidence, RuntimeCheckpoint: checkpoint.Checkpoint, PublicPrefix: checkpoint.PublicPrefix, ReasonEvidence: []run.EvidenceRef{parentEvidence}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := experimentOp.BindPreparedRun("child", origin, prepared); err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	host, err := NewPiRuntimeHost([]PiRuntimeProfile{
		{Ref: "parent-profile", ExperimentID: "exp", RunID: "worker", Role: effect.WorkerStart, SessionPath: parentSession, Identity: testIdentity(t), Policy: policy, WorkerLaunch: parent},
		{Ref: "child-profile", ExperimentID: "exp", RunID: "child", Role: effect.WorkerStart, SessionPath: childSession, Identity: testIdentity(t), Policy: policy, WorkerLaunch: child},
	})
	if err != nil {
		t.Fatal(err)
	}
	binding.Runtime = host
	value, err := host.RuntimeTree(binding, "child", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	page := value.(PiRuntimeTreePage)
	if len(page.Entries) != 1 || page.Entries[0].Evidence != parentEvidence {
		t.Fatalf("spliced runtime tree evidence = %#v, want %#v", page, parentEvidence)
	}
}
