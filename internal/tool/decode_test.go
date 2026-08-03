package tool

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/source"
)

func TestDecodeAcceptsOnlyClosedFourToolOperations(t *testing.T) {
	tests := []struct {
		name, tool, input string
	}{
		{"apply", ApplyTool, `{"action":"begin_preparation","user_intent":{"algorithm":"sha256","digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1},"source_snapshot":{"algorithm":"sha256","digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","size":1}}`},
		{"run", RunTool, `{"action":"status","run_id":"r1"}`},
		{"inspect", InspectTool, `{"scope":"experiment","after":0,"limit":1}`},
		{"compare", CompareTool, `{"action":"show","comparison_id":"c1"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := Decode(test.tool, []byte(test.input))
			if err != nil || value.toolName() != test.tool {
				t.Fatalf("decode = %#v, %v", value, err)
			}
		})
	}
}

func TestDecodeRejectsLocatorsAndIllegalProducts(t *testing.T) {
	tests := []struct{ tool, input string }{
		{ApplyTool, `{"action":"begin_preparation","root":"/state"}`},
		{RunTool, `{"action":"start","request_path":"/tmp/request.json"}`},
		{RunTool, `{"action":"poll","run_id":"r","stream_path":"/tmp/session"}`},
		{InspectTool, `{"scope":"run","after":0,"limit":1}`},
		{InspectTool, `{"scope":"experiment","run_id":"r","after":0,"limit":1}`},
		{CompareTool, `{"action":"show","gate_id":"g"}`},
		{"agentlab_prepare", `{"action":"status"}`},
	}
	for _, test := range tests {
		if _, err := Decode(test.tool, []byte(test.input)); err == nil {
			t.Fatalf("accepted %s %s", test.tool, test.input)
		}
	}
}

func TestExecuteUsesHostBoundRefsWithoutFilesystemInput(t *testing.T) {
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
	input, err := json.Marshal(map[string]any{"action": "begin_preparation", "user_intent": intent, "source_snapshot": snapshot})
	if err != nil {
		t.Fatal(err)
	}
	value, err := Decode(ApplyTool, input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(Binding{Root: root, PreparationID: "prep"}, value)
	if err != nil {
		t.Fatal(err)
	}
	status, ok := result.(preparation.Status)
	if !ok || status.Phase != preparation.PhaseExploring || status.WorkerInput.Digest == "" {
		t.Fatalf("result = %#v", result)
	}
	if strings.Contains(string(input), root) {
		t.Fatal("model input contains host root")
	}
}
