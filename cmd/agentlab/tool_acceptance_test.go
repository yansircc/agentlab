package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yansircc/agentlab/internal/preparation"
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

func TestToolInvokeDecodesThenDispatches(t *testing.T) {
	root := t.TempDir()
	files := t.TempDir()
	intent := writeToolFile(t, files, "intent.txt", []byte("intent"))
	source := writeToolFile(t, files, "source.txt", []byte("source"))
	request := writeJSONFile(t, files, "begin.json", map[string]any{
		"preparation_id": "tool-prep", "user_intent_path": intent,
		"source_files": []map[string]any{{"path": "source.txt", "content_path": source}}, "authority": "designer",
	})
	input := writeJSONFile(t, files, "tool.json", map[string]any{
		"action": "begin", "root": root, "request_path": request,
	})
	result, err := dispatch([]string{"tool", "invoke", "-name", "agentlab_prepare", "-input", input})
	if err != nil {
		t.Fatal(err)
	}
	status, ok := result.(preparation.Status)
	if !ok || status.PreparationID != "tool-prep" || status.Phase != preparation.PhaseExploring {
		t.Fatalf("tool invoke = %#v", result)
	}
}

func writeToolFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
