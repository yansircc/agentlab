package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/gate"
)

type DecisionBoundComparison struct {
	Decision    SupervisorDecision     `json:"decision"`
	Observation comparison.Observation `json:"observation"`
}

type DecisionBoundGate struct {
	Decision SupervisorDecision `json:"decision"`
	Receipt  gate.Receipt       `json:"receipt"`
}

func (value DecisionBoundComparison) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionComparison || value.Observation.Validate() != nil {
		return errors.New("decision-bound comparison is invalid")
	}
	return nil
}

func (value DecisionBoundGate) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionGate || value.Receipt.Validate() != nil {
		return errors.New("decision-bound gate is invalid")
	}
	return nil
}

func (o *Operation) CompareWithDecision(value DecisionBoundComparison) (comparison.Result, error) {
	if value.Validate() != nil {
		return comparison.Result{}, errors.New("decision-bound comparison is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return comparison.Result{}, err
	}
	return o.compare(value.Observation, &value.Decision)
}

func (o *Operation) RecordGateWithDecision(value DecisionBoundGate) (gate.Result, error) {
	if value.Validate() != nil {
		return gate.Result{}, errors.New("decision-bound gate is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return gate.Result{}, err
	}
	return o.recordGate(value.Receipt.Spec, &value.Decision)
}
