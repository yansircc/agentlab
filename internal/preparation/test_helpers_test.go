package preparation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func assertWorkerInputBoundary(t *testing.T, op *Operation, status Status) {
	t.Helper()
	data, err := op.artifacts.Read(status.WorkerInput)
	if err != nil {
		t.Fatal(err)
	}
	var input WorkerInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	if input.Contract != workerInputContract || input.UserIntentRef == status.Source || strings.Contains(string(data), "private-source-facts") {
		t.Fatalf("private source crossed worker input boundary: %s", data)
	}
}

func begunOperation(t *testing.T, id string) *Operation {
	t.Helper()
	op, _ := Open(t.TempDir(), id)
	if _, err := op.Begin(BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "source.txt", Content: []byte("source")}}, Authority: "designer"}); err != nil {
		t.Fatal(err)
	}
	return op
}

func partialBegin(t *testing.T, id string) (*Operation, BeginSpec) {
	t.Helper()
	op, _ := Open(t.TempDir(), id)
	spec := BeginSpec{UserIntent: []byte("intent"), SourceFiles: []source.InputFile{{Path: "source.txt", Content: []byte("source")}}, Authority: "designer"}
	intent, _ := op.artifacts.Put(spec.UserIntent)
	inputBytes, _ := json.Marshal(WorkerInput{Contract: workerInputContract, UserIntentRef: intent, PublicArtifacts: []artifact.Ref{}})
	input, _ := op.artifacts.Put(inputBytes)
	if _, err := op.ledger.Append(time.Now().UTC(), eventWorkerInput, inputSealed{UserIntent: intent, WorkerInput: input, Authority: spec.Authority}); err != nil {
		t.Fatal(err)
	}
	return op, spec
}

func recordCleanAssay(t *testing.T, op *Operation) {
	t.Helper()
	status, err := op.Status()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := op.artifacts.Put([]byte("independent semantic contrast found no source-derived instructions"))
	if err != nil {
		t.Fatal(err)
	}
	if err := op.RecordLeakageAssay(LeakageAssay{
		WorkerInput: status.WorkerInput, SourceSnapshot: status.Source,
		Reviewer: "independent-reviewer", Authority: "reviewer", Method: "semantic-contrast-review",
		Verdict: LeakageClean, Evidence: []artifact.Ref{evidence},
	}); err != nil {
		t.Fatal(err)
	}
}
