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

// BindPreparedRunWithDecision accepts the complete Host-issued input set as
// one opaque fact. Supervisor input still owns only its decision and origin.
func (o *Operation) BindPreparedRunWithDecision(value DecisionBoundRunBinding, origin RunOrigin, preparedRef artifact.Ref) (artifact.Ref, error) {
	if value.Validate() != nil || !preparedRef.Valid() {
		return artifact.Ref{}, errors.New("decision-bound prepared run is invalid")
	}
	prepared, err := LoadPreparedRun(o.artifacts, preparedRef)
	if err != nil || prepared.RunID != value.RunID {
		return artifact.Ref{}, errors.New("prepared run differs from binding")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return artifact.Ref{}, err
	}
	return o.bindRun(value.RunID, origin, prepared.Inputs, &value.Decision)
}

type decisionRunRecord struct {
	Decision SupervisorDecision `json:"decision"`
	Binding  runBinding         `json:"binding"`
}
