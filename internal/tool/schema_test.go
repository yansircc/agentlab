package tool

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProviderProjectionsHaveExactlyFourLocatorFreeTools(t *testing.T) {
	anthropic, err := Projection("anthropic")
	if err != nil {
		t.Fatal(err)
	}
	openAI, err := Projection("openai_responses")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{ApplyTool, RunTool, InspectTool, CompareTool}
	if len(anthropic) != len(want) || len(openAI) != len(want) {
		t.Fatalf("tool count = %d/%d", len(anthropic), len(openAI))
	}
	for index, name := range want {
		if anthropic[index]["name"] != name || openAI[index]["name"] != name || !reflect.DeepEqual(anthropic[index]["input_schema"], openAI[index]["parameters"]) {
			t.Fatalf("projection %d differs", index)
		}
		assertNoLocator(t, anthropic[index]["input_schema"])
	}
}

func TestProjectionRejectsUnknownProvider(t *testing.T) {
	if _, err := Projection("openai"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func assertNoLocator(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"root", "request_path", "stream_path", "session_path", "executable_path", "raw_transcript", "audit_root"} {
		if string(data) == banned || containsJSONKey(value, banned) {
			t.Fatalf("schema exposes %q: %s", banned, data)
		}
	}
}

func containsJSONKey(value any, want string) bool {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if key == want || containsJSONKey(child, want) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if containsJSONKey(child, want) {
				return true
			}
		}
	}
	return false
}
