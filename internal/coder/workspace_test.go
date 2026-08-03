package coder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func TestWorkspaceMaterializesOnlyExactSnapshotAndSealsCandidate(t *testing.T) {
	root := t.TempDir()
	store := artifact.NewStore(filepath.Join(root, "artifacts"))
	snapshot, err := source.Build(store, []source.InputFile{{Path: "cmd/main.go", Content: []byte("baseline")}})
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(root, "worker.jsonl")
	audit := filepath.Join(root, "audit")
	if err := os.WriteFile(worker, []byte("worker-private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(audit, 0o700); err != nil {
		t.Fatal(err)
	}
	workspaceRoot := filepath.Join(root, "workspace")
	receipt, err := Prepare(store, snapshot, workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(store, receipt, snapshot, workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Read("../worker.jsonl"); err == nil {
		t.Fatal("coder read escaped to worker session")
	}
	if _, err := workspace.Read(audit); err == nil {
		t.Fatal("coder read accepted audit root")
	}
	if err := workspace.Write("cmd/main.go", []byte("repaired")); err != nil {
		t.Fatal(err)
	}
	candidate, err := workspace.Seal(store)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := source.Load(store, candidate)
	if err != nil || len(sealed.Files) != 1 || sealed.Files[0].Path != "cmd/main.go" {
		t.Fatalf("candidate = %#v, %v", sealed, err)
	}
}

func TestWorkspaceRejectsDriftAndSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	store := artifact.NewStore(filepath.Join(root, "artifacts"))
	snapshot, err := source.Build(store, []source.InputFile{{Path: "safe.txt", Content: []byte("safe")}})
	if err != nil {
		t.Fatal(err)
	}
	workspaceRoot := filepath.Join(root, "workspace")
	receipt, err := Prepare(store, snapshot, workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "extra.txt"), []byte("drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(store, receipt, snapshot, workspaceRoot); err == nil {
		t.Fatal("workspace drift was accepted")
	}
	if err := os.Remove(filepath.Join(workspaceRoot, "extra.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "artifacts"), filepath.Join(workspaceRoot, "outside")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(store, receipt, snapshot, workspaceRoot); err == nil {
		t.Fatal("workspace symlink was accepted")
	}
}
