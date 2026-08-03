package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/preparation"
)

func TestPreparationCLIEndToEnd(t *testing.T) {
	root := t.TempDir()
	files := t.TempDir()
	intent := filepath.Join(files, "intent.txt")
	source := filepath.Join(files, "source.txt")
	if err := os.WriteFile(intent, []byte("public intent"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("private source snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	beginPath := writeJSONFile(t, files, "begin.json", map[string]any{
		"preparation_id": "cli-prep", "user_intent_path": intent,
		"source_files": []map[string]any{{"path": "source.txt", "content_path": source}}, "authority": "designer",
	})
	result, err := dispatch([]string{"prepare", "begin", "-root", root, "-request", beginPath})
	if err != nil {
		t.Fatal(err)
	}
	status, ok := result.(preparation.Status)
	if !ok || status.Phase != preparation.PhaseExploring {
		t.Fatalf("begin status = %#v", result)
	}
	assayEvidence, _ := artifact.NewStore(filepath.Join(root, "artifacts")).Put([]byte("independent semantic leakage assay"))
	assayPath := writeJSONFile(t, files, "assay.json", map[string]any{
		"preparation_id": "cli-prep", "assay": preparation.LeakageAssay{
			WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "reviewer-1", Authority: "reviewer",
			Method: "semantic-contrast-review", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{assayEvidence},
		},
	})
	if _, err := dispatch([]string{"prepare", "assay", "-root", root, "-request", assayPath}); err != nil {
		t.Fatal(err)
	}
	basisResult, err := dispatch([]string{"prepare", "challenge-basis", "-root", root, "-preparation", "cli-prep"})
	if err != nil {
		t.Fatal(err)
	}
	basis := basisResult.(artifact.Ref)
	challengePath := writeJSONFile(t, files, "challenge.json", map[string]any{
		"preparation_id": "cli-prep", "challenge": map[string]any{"basis": basis, "gaps": []any{}},
	})
	if _, err := dispatch([]string{"prepare", "challenge", "-root", root, "-request", challengePath}); err != nil {
		t.Fatal(err)
	}
	sealedResult, err := dispatch([]string{"prepare", "seal", "-root", root, "-preparation", "cli-prep"})
	if err != nil || sealedResult.(preparation.Status).Phase != preparation.PhaseSealed {
		t.Fatalf("seal = %#v, %v", sealedResult, err)
	}
	page, err := dispatch([]string{"inspect", "-root", root, "-preparation", "cli-prep", "-after", "0", "-limit", "2"})
	records, ok := page.([]ledger.Record)
	if err != nil || !ok || len(records) != 2 {
		t.Fatalf("preparation inspect = %#v, %v", page, err)
	}
}

func TestPreparationCLIRejectsUnknownRequestFields(t *testing.T) {
	path := writeJSONFile(t, t.TempDir(), "unknown.json", map[string]any{
		"preparation_id": "bad", "shadow": true,
	})
	if _, err := dispatch([]string{"prepare", "record-fact", "-root", t.TempDir(), "-request", path}); err == nil {
		t.Fatal("unknown request field was accepted")
	}
}

func writeJSONFile(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
