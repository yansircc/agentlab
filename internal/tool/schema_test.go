package tool

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProviderProjectionsHaveOneEquivalentSchemaPerTool(t *testing.T) {
	anthropic, err := Projection("anthropic")
	if err != nil {
		t.Fatal(err)
	}
	openAI, err := Projection("openai_responses")
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{PrepareTool, RunTool, InspectTool, CompareTool}
	if len(anthropic) != len(wantNames) || len(openAI) != len(wantNames) {
		t.Fatalf("tool count = %d/%d, want %d", len(anthropic), len(openAI), len(wantNames))
	}
	for index, name := range wantNames {
		if anthropic[index]["name"] != name || openAI[index]["name"] != name {
			t.Fatalf("tool %d names = %v/%v, want %q", index, anthropic[index]["name"], openAI[index]["name"], name)
		}
		if !reflect.DeepEqual(anthropic[index]["input_schema"], openAI[index]["parameters"]) {
			t.Fatalf("provider schemas differ for %s", name)
		}
		assertSimpleSchema(t, anthropic[index]["input_schema"])
	}
}

func TestProjectionRejectsUnknownProvider(t *testing.T) {
	if _, err := Projection("openai"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func assertSimpleSchema(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "oneOf" || key == "anyOf" || key == "allOf" || key == "$ref" {
					t.Fatalf("unsupported schema keyword %q in %s", key, data)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
}
