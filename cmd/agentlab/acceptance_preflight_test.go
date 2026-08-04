package main

import (
	"path/filepath"
	"testing"
)

func TestCLIAcceptanceProvisionReturnsOnlyOpaqueEvaluatedRefs(t *testing.T) {
	parent := t.TempDir()
	result, err := dispatch([]string{"acceptance", "provision", "-evaluated-root", filepath.Join(parent, "evaluated"), "-audit-root", filepath.Join(parent, "audit")})
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.(acceptanceProvisionProjection)
	if !ok || value.ExperimentID != "deployctl-supervision" || value.BaselineRunID != "baseline-worker" || !value.WorkerInput.Valid() || !value.Candidate.Valid() || !value.CandidateExecutable.Valid() {
		t.Fatalf("acceptance provision = %#v", result)
	}
	if value.WorkerInput.Scope != value.Candidate.Scope || value.WorkerInput.Scope == "" {
		t.Fatalf("provision projection crossed capability roots: %#v", value)
	}
}
