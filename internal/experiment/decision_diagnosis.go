package experiment

import (
	"errors"
	"fmt"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/diagnosis"
)

type DecisionBoundDiagnosis struct {
	Decision  SupervisorDecision  `json:"decision"`
	Diagnosis diagnosis.Diagnosis `json:"diagnosis"`
}

type DecisionBoundCandidate struct {
	Decision      SupervisorDecision `json:"decision"`
	ID            string             `json:"id"`
	DiagnosisID   string             `json:"diagnosis_id"`
	CoderRun      string             `json:"coder_run"`
	CompletionRef artifact.Ref       `json:"completion_ref"`
}

type decisionBoundCandidateRecorded struct {
	Decision  SupervisorDecision        `json:"decision"`
	Candidate diagnosis.RepairCandidate `json:"candidate"`
}

func (value DecisionBoundDiagnosis) Validate() error {
	if err := value.Decision.Validate(); err != nil {
		return err
	}
	if value.Decision.Action != DecisionDiagnosis {
		return errors.New("decision-bound diagnosis requires the diagnosis action")
	}
	if err := value.Diagnosis.Validate(); err != nil {
		return fmt.Errorf("decision-bound diagnosis: %w", err)
	}
	return nil
}

func (value DecisionBoundCandidate) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionCandidate || !idPattern.MatchString(value.ID) || !idPattern.MatchString(value.DiagnosisID) || !idPattern.MatchString(value.CoderRun) || !value.CompletionRef.Valid() {
		return errors.New("decision-bound candidate is invalid")
	}
	return nil
}

func (o *Operation) RecordDiagnosisWithDecision(value DecisionBoundDiagnosis) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return err
	}
	if err := o.validateDiagnosis(value.Diagnosis); err != nil {
		return err
	}
	return o.recordDiagnosis(value.Diagnosis, &value.Decision)
}

func (o *Operation) BindCandidateWithDecision(value DecisionBoundCandidate) (diagnosis.RepairCandidate, error) {
	if err := value.Validate(); err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	return o.bindCandidate(value.ID, value.DiagnosisID, value.CoderRun, value.CompletionRef, &value.Decision)
}
