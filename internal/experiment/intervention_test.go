package experiment

import (
	"testing"
	"time"
)

func TestInterventionRequiresCanonicalExperimentOwnedArtifact(t *testing.T) {
	_, operation, _, effect := decisionFixture(t)
	unowned, err := operation.artifacts.PutCanonicalJSON([]byte(`{"contract":"agentlab.intervention.v1","text":"unowned"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operation.Intervention(unowned); err == nil {
		t.Fatal("unowned intervention was accepted")
	}

	decision := effect.Decision
	decision.ID, decision.Action = "intervention-owned", DecisionIntervention
	owned, err := operation.RecordInterventionWithDecision(DecisionBoundIntervention{
		Decision: decision, Intervention: Intervention{Contract: InterventionContract, Text: "re-observe the public contract"},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := operation.Intervention(owned)
	if err != nil || value.Text != "re-observe the public contract" {
		t.Fatalf("owned intervention = %#v, %v", value, err)
	}

	// Structurally valid JSON in a different byte order is not a canonical
	// Intervention artifact and cannot be admitted through replay either.
	nonCanonical, err := operation.artifacts.Put([]byte(`{"text":"noncanonical","contract":"agentlab.intervention.v1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIntervention(operation.artifacts, nonCanonical); err == nil {
		t.Fatal("non-canonical intervention artifact was accepted")
	}
	decision.ID = "intervention-noncanonical"
	if _, err := operation.ledger.Append(time.Now().UTC(), eventDecisionIntervention, decisionInterventionRecorded{
		Decision: decision, Artifact: nonCanonical, Intervention: Intervention{Contract: InterventionContract, Text: "noncanonical"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := operation.Status(); err == nil {
		t.Fatal("replay accepted a non-canonical intervention artifact")
	}
}
