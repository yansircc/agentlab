package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

type DecisionBoundRunBinding struct {
	Decision SupervisorDecision `json:"decision"`
	RunID    string             `json:"run_id"`
}

func (value DecisionBoundRunBinding) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionRunBinding || !idPattern.MatchString(value.RunID) {
		return errors.New("decision-bound run binding is invalid")
	}
	return nil
}

func (o *Operation) BindRunWithDecision(value DecisionBoundRunBinding, origin RunOrigin, inputs RunInputs) (artifact.Ref, error) {
	if value.Validate() != nil {
		return artifact.Ref{}, errors.New("decision-bound run binding is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return artifact.Ref{}, err
	}
	return o.bindRun(value.RunID, origin, inputs, &value.Decision)
}

type decisionRunRecord struct {
	Decision SupervisorDecision `json:"decision"`
	Binding  runBinding         `json:"binding"`
}
