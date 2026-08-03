package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/finding"
)

func (o *Operation) Status() (Status, error) {
	current, err := o.current()
	if err != nil {
		return Status{}, err
	}
	status := Status{ExperimentID: o.id, EventCount: current.eventCount, FindingIDs: append([]string(nil), current.order...), DispositionCount: len(current.dispositions)}
	status.DiagnosisIDs = append([]string(nil), current.diagnosisOrder...)
	status.CandidateIDs = append([]string(nil), current.candidateOrder...)
	status.RunIDs = append([]string(nil), current.runOrder...)
	status.ComparisonIDs = append([]string(nil), current.comparisonOrder...)
	status.GateIDs = append([]string(nil), current.gateOrder...)
	status.DecisionIDs = append([]string(nil), current.decisionOrder...)
	if current.begun != nil {
		status.PreparationID, status.WorkerInput = current.begun.PreparationID, current.begun.WorkerInput
	}
	return status, nil
}

func (o *Operation) Finding(id string) (finding.Finding, *finding.Disposition, error) {
	current, err := o.current()
	if err != nil {
		return finding.Finding{}, nil, err
	}
	value := current.findings[id]
	if value.ID == "" {
		return finding.Finding{}, nil, errors.New("finding does not exist")
	}
	disposition := current.dispositions[id]
	if disposition.FindingID == "" {
		return value, nil, nil
	}
	return value, &disposition, nil
}
