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

func TestCLIAcceptancePrepareRunRejectsRawCandidateInput(t *testing.T) {
	request := writeJSONFile(t, t.TempDir(), "prepare-run.json", map[string]any{
		"run_id": "candidate", "completion": map[string]any{}, "candidate": map[string]any{},
	})
	if _, err := dispatch([]string{"acceptance", "prepare-run", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("Host producer command accepted a raw candidate input")
	}
}

func TestCLIAcceptancePrepareBaselineRejectsRawCandidateInput(t *testing.T) {
	request := writeJSONFile(t, t.TempDir(), "prepare-baseline.json", map[string]any{
		"run_id": "baseline-repeat", "candidate": map[string]any{},
	})
	if _, err := dispatch([]string{"acceptance", "prepare-baseline", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("baseline Host producer accepted a raw candidate input")
	}
}

func TestCLIAcceptanceHeldoutRejectsRawCandidateInput(t *testing.T) {
	request := writeJSONFile(t, t.TempDir(), "heldout.json", map[string]any{
		"prepared": map[string]any{}, "candidate": map[string]any{},
	})
	if _, err := dispatch([]string{"acceptance", "verify-heldout", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("held-out Host check accepted a raw candidate input")
	}
}

func TestCLIAcceptanceAuditCommandsRejectProviderFields(t *testing.T) {
	request := writeJSONFile(t, t.TempDir(), "audit-review.json", map[string]any{
		"review": map[string]any{}, "root": "/provider-selected", "candidate": map[string]any{},
	})
	if _, err := dispatch([]string{"acceptance", "audit-review", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("Host/Codex audit command accepted provider-selected fields")
	}
	if _, err := dispatch([]string{"acceptance", "audit-seal", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("audit seal accepted an unexpected request path")
	}
	if _, err := dispatch([]string{"acceptance", "audit-intervened", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("audit intervention accepted an evaluated request")
	}
}

func TestCLIAcceptanceSupervisorCommandsRejectProviderFields(t *testing.T) {
	request := writeJSONFile(t, t.TempDir(), "supervisor-start.json", map[string]any{
		"root": "/provider-selected", "runtime_plan": "/provider-selected/plan.json",
	})
	if _, err := dispatch([]string{"acceptance", "supervisor-start", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("Host Supervisor launcher accepted a provider-selected request path")
	}
	if _, err := dispatch([]string{"acceptance", "supervisor-status", "-host-root", t.TempDir(), "-request", request}); err == nil {
		t.Fatal("Host Supervisor status accepted a provider-selected request path")
	}
}
