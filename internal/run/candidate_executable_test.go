package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func TestCandidateExecutableBindsOneSealedSourceAndExactBytes(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	candidate, err := source.Build(store, []source.InputFile{{Path: "main.go", Content: []byte("package main")}})
	if err != nil {
		t.Fatal(err)
	}
	binary, err := store.Put([]byte("binary"))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := BindCandidateExecutable(store, candidate, binary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(path, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCandidateExecutable(store, receipt, candidate, path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("drift"), 0o700); err != nil {
		t.Fatal(err)
	}
	if VerifyCandidateExecutable(store, receipt, candidate, path) == nil {
		t.Fatal("drifted executable was accepted")
	}
}
