package experiment

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const InterventionContract = "agentlab.intervention.v1"

// Intervention is the immutable, model-visible information added only to a
// splice child. Its supporting evidence belongs to the enclosing decision;
// the artifact itself is the single source for the child context content.
type Intervention struct {
	Contract string `json:"contract"`
	Text     string `json:"text"`
}

type DecisionBoundIntervention struct {
	Decision     SupervisorDecision `json:"decision"`
	Intervention Intervention       `json:"intervention"`
}

type decisionInterventionRecorded struct {
	Decision     SupervisorDecision `json:"decision"`
	Artifact     artifact.Ref       `json:"artifact"`
	Intervention Intervention       `json:"intervention"`
}

func (value Intervention) Validate() error {
	if value.Contract != InterventionContract || strings.TrimSpace(value.Text) == "" || !utf8.ValidString(value.Text) || len(value.Text) > 32768 {
		return errors.New("intervention is invalid")
	}
	return nil
}

func (value DecisionBoundIntervention) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionIntervention || value.Intervention.Validate() != nil {
		return errors.New("decision-bound intervention is invalid")
	}
	return nil
}

// LoadIntervention reads the only artifact form permitted to add information
// to a splice child. It rejects non-canonical bytes as well as any schema or
// contract drift, so a generic artifact cannot masquerade as Intervention.
func LoadIntervention(store artifact.Store, ref artifact.Ref) (Intervention, error) {
	if !ref.Valid() {
		return Intervention{}, errors.New("intervention reference is invalid")
	}
	data, err := store.Read(ref)
	if err != nil {
		return Intervention{}, err
	}
	canonical, err := artifact.CanonicalJSON(data)
	if err != nil || !bytes.Equal(data, canonical) {
		return Intervention{}, errors.New("intervention artifact is not canonical")
	}
	var value Intervention
	if strictjson.Decode(data, &value) != nil || value.Validate() != nil {
		return Intervention{}, errors.New("intervention artifact is invalid")
	}
	return value, nil
}

// RecordInterventionWithDecision is the sole producer for a Supervisor's new
// child-context information. It stores canonical immutable bytes and records
// the decision/artifact pairing atomically in the experiment ledger.
func (o *Operation) RecordInterventionWithDecision(value DecisionBoundIntervention) (artifact.Ref, error) {
	if value.Validate() != nil {
		return artifact.Ref{}, errors.New("decision-bound intervention is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return artifact.Ref{}, err
	}
	data, err := json.Marshal(value.Intervention)
	if err != nil {
		return artifact.Ref{}, err
	}
	ref, err := o.artifacts.PutCanonicalJSON(data)
	if err != nil {
		return artifact.Ref{}, err
	}
	err = o.mutate(func(current *state) error {
		if current.begun == nil || current.decisions[value.Decision.ID].ID != "" || current.interventions[ref].Contract != "" {
			return errors.New("decision-bound intervention identity already exists")
		}
		return o.append(eventDecisionIntervention, decisionInterventionRecorded{Decision: value.Decision, Artifact: ref, Intervention: value.Intervention})
	})
	return ref, err
}

// Intervention returns an artifact only after both its canonical content and
// its decision-bound ownership in this experiment have been verified.
func (o *Operation) Intervention(ref artifact.Ref) (Intervention, error) {
	current, err := o.current()
	if err != nil {
		return Intervention{}, err
	}
	want, ok := current.interventions[ref]
	if !ok {
		return Intervention{}, errors.New("intervention is not experiment-owned")
	}
	value, err := LoadIntervention(o.artifacts, ref)
	if err != nil || value != want {
		return Intervention{}, errors.New("intervention artifact is invalid")
	}
	return value, nil
}

func (o *Operation) validateInterventions(current state) error {
	for ref, want := range current.interventions {
		got, err := LoadIntervention(o.artifacts, ref)
		if err != nil || got != want {
			return errors.New("intervention artifact is invalid")
		}
	}
	return nil
}
