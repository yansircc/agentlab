package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/diagnosis"
)

type DecisionBoundDiagnosis struct {
	Decision  SupervisorDecision  `json:"decision"`
	Diagnosis diagnosis.Diagnosis `json:"diagnosis"`
}

type DecisionBoundCandidate struct {
	Decision  SupervisorDecision        `json:"decision"`
	Candidate diagnosis.RepairCandidate `json:"candidate"`
}

func (value DecisionBoundDiagnosis) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionDiagnosis || value.Diagnosis.Validate() != nil {
		return errors.New("decision-bound diagnosis is invalid")
	}
	return nil
}

func (value DecisionBoundCandidate) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionCandidate || value.Candidate.Validate() != nil {
		return errors.New("decision-bound candidate is invalid")
	}
	return nil
}

func (o *Operation) RecordDiagnosisWithDecision(value DecisionBoundDiagnosis) error {
	if value.Validate() != nil {
		return errors.New("decision-bound diagnosis is invalid")
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
	if value.Validate() != nil {
		return diagnosis.RepairCandidate{}, errors.New("decision-bound candidate is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	return o.bindCandidate(value.Candidate.ID, value.Candidate.DiagnosisID, value.Candidate.Artifact, &value.Decision)
}
