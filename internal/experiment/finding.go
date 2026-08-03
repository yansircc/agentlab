package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/run"
)

func (o *Operation) RecordFinding(value finding.Finding) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := o.validateFindingEvidence(value); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		if current.findings[value.ID].ID != "" {
			return errors.New("finding id already exists")
		}
		return o.append(eventFinding, value)
	})
}

func (o *Operation) RecordFindingWithDecision(value DecisionBoundFinding) error {
	if value.Validate() != nil {
		return errors.New("decision-bound finding is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return err
	}
	if err := o.validateFindingEvidence(value.Finding); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		if current.findings[value.Finding.ID].ID != "" || current.decisions[value.Decision.ID].ID != "" {
			return errors.New("decision-bound finding identity already exists")
		}
		return o.append(eventDecisionFinding, value)
	})
}

func (o *Operation) validateFindingEvidence(value finding.Finding) error {
	for _, ref := range value.Evidence {
		if ref.ExperimentID != o.id {
			return errors.New("finding evidence belongs to another experiment")
		}
		runOperation, err := run.Open(o.root, o.id, ref.RunID)
		if err != nil {
			return err
		}
		if _, err := runOperation.EvidenceAt(ref); err != nil {
			return err
		}
	}
	return nil
}

func (o *Operation) RecordDisposition(value finding.Disposition) error {
	if err := value.Validate(); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		if current.findings[value.FindingID].ID == "" {
			return errors.New("finding does not exist")
		}
		if current.dispositions[value.FindingID].FindingID != "" {
			return errors.New("finding already has a disposition")
		}
		return o.append(eventDisposition, value)
	})
}
