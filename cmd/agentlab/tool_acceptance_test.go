package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/source"
)

func TestToolSchemaCLIProjectsExactlyFourTools(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai_responses"} {
		result, err := dispatch([]string{"tool", "schemas", "-provider", provider})
		if err != nil {
			t.Fatal(err)
		}
		definitions, ok := result.([]map[string]any)
		if !ok || len(definitions) != 4 {
			t.Fatalf("%s schemas = %#v", provider, result)
		}
	}
}

func TestToolInvokeReadsHostBoundStdin(t *testing.T) {
	root := t.TempDir()
	store := artifact.NewStore(root + "/artifacts")
	intent, err := store.Put([]byte("intent"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.Build(store, []source.InputFile{{Path: "source.txt", Content: []byte("source")}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"action": "begin_preparation", "user_intent": intent, "source_snapshot": snapshot})
	if err != nil {
		t.Fatal(err)
	}
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = previous; _ = input.Close() }()
	result, err := dispatch([]string{"tool", "invoke", "-name", "agentlab_apply", "-root", root, "-preparation", "tool-prep"})
	if err != nil {
		t.Fatal(err)
	}
	status, ok := result.(preparation.Status)
	if !ok || status.PreparationID != "tool-prep" || status.Phase != preparation.PhaseExploring {
		t.Fatalf("tool invoke = %#v", result)
	}
}
