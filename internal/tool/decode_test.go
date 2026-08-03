package tool

import (
	"reflect"
	"testing"
)

func TestDecodeMapsValidInputsToCanonicalCLI(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input string
		want  []string
	}{
		{"prepare mutation", PrepareTool, `{"action":"record_fact","root":"/state","request_path":"/tmp/fact.json"}`, []string{"prepare", "record-fact", "-root", "/state", "-request", "/tmp/fact.json"}},
		{"prepare assay", PrepareTool, `{"action":"assay","request_path":"/tmp/assay.json"}`, []string{"prepare", "assay", "-request", "/tmp/assay.json"}},
		{"prepare status", PrepareTool, `{"action":"status","preparation_id":"p1"}`, []string{"prepare", "status", "-preparation", "p1"}},
		{"owned run", RunTool, `{"action":"start","experiment_id":"e1","run_id":"r1","request_path":"/tmp/start.json","first_event":"2s","soft_idle":"3s","hard_idle":"4s","kill_on_hard_idle":true}`, []string{"run", "start", "-experiment", "e1", "-run", "r1", "-request", "/tmp/start.json", "-first-event", "2s", "-soft-idle", "3s", "-hard-idle", "4s", "-kill-on-hard-idle"}},
		{"attach begin", RunTool, `{"action":"attach_begin","experiment_id":"e1","run_id":"r1","adapter":"pi","stream_path":"/tmp/session.jsonl","first_event":"2s","soft_idle":"3s","hard_idle":"4s"}`, []string{"run", "attach", "begin", "-experiment", "e1", "-run", "r1", "-adapter", "pi", "-stream", "/tmp/session.jsonl", "-first-event", "2s", "-soft-idle", "3s", "-hard-idle", "4s"}},
		{"bounded inspect", InspectTool, `{"scope":"run","experiment_id":"e1","run_id":"r1","after":7,"limit":20}`, []string{"inspect", "-experiment", "e1", "-run", "r1", "-after", "7", "-limit", "20"}},
		{"comparison", CompareTool, `{"action":"show","experiment_id":"e1","comparison_id":"c1"}`, []string{"compare", "show", "-experiment", "e1", "-comparison", "c1"}},
		{"candidate gate", CompareTool, `{"action":"gate_show","experiment_id":"e1","gate_id":"g1"}`, []string{"gate", "show", "-experiment", "e1", "-gate", "g1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Decode(test.tool, []byte(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Args, test.want) {
				t.Fatalf("args = %#v, want %#v", got.Args, test.want)
			}
		})
	}
}

func TestDecodeRejectsIllegalProducts(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input string
	}{
		{"unknown field", InspectTool, `{"scope":"run","experiment_id":"e","run_id":"r","after":0,"limit":1,"secret":"x"}`},
		{"trailing value", CompareTool, `{"action":"show","experiment_id":"e","comparison_id":"c"}{}`},
		{"prepare mixed target", PrepareTool, `{"action":"seal","preparation_id":"p","request_path":"/tmp/x"}`},
		{"missing start request", RunTool, `{"action":"start","experiment_id":"e","run_id":"r"}`},
		{"start missing deadlines", RunTool, `{"action":"start","experiment_id":"e","run_id":"r","request_path":"/tmp/start.json","first_event":"1s"}`},
		{"attach request", RunTool, `{"action":"attach_begin","experiment_id":"e","run_id":"r","adapter":"pi","stream_path":"/tmp/s","request_path":"/tmp/start.json","first_event":"1s","soft_idle":"2s","hard_idle":"3s"}`},
		{"unbounded inspect", InspectTool, `{"scope":"run","experiment_id":"e","run_id":"r"}`},
		{"missing inspect cursor", InspectTool, `{"scope":"run","experiment_id":"e","run_id":"r","limit":1}`},
		{"oversized inspect", InspectTool, `{"scope":"run","experiment_id":"e","run_id":"r","after":0,"limit":1001}`},
		{"record with ids", CompareTool, `{"action":"record","request_path":"/tmp/c","experiment_id":"e"}`},
		{"gate show with comparison", CompareTool, `{"action":"gate_show","experiment_id":"e","gate_id":"g","comparison_id":"c"}`},
		{"stdin request", CompareTool, `{"action":"record","request_path":"-"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Decode(test.tool, []byte(test.input)); err == nil {
				t.Fatal("invalid tool input accepted")
			}
		})
	}
}
