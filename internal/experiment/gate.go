package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/run"
)

func (o *Operation) RecordGate(spec gate.Spec) (gate.Result, error) {
	if err := spec.Validate(); err != nil {
		return gate.Result{}, err
	}
	if err := o.requireSettledEffects(); err != nil {
		return gate.Result{}, err
	}
	for _, item := range spec.Items {
		for _, ref := range item.Evidence {
			if err := o.validateGateEvidence(ref); err != nil {
				return gate.Result{}, err
			}
		}
	}
	var result gate.Result
	err := o.mutate(func(current *state) error {
		candidate := current.candidates[spec.CandidateID]
		if candidate.ID == "" {
			return errors.New("gate candidate does not exist")
		}
		if current.gates[spec.ID].ID != "" {
			return errors.New("gate id already exists")
		}
		for _, blocker := range spec.BlockerFindings() {
			if current.findings[blocker.ID].ID != "" {
				return errors.New("gate blocker finding id already exists")
			}
		}
		if err := o.validateGateComparison(current, spec); err != nil {
			return err
		}
		receipt := gate.Receipt{Spec: spec, Candidate: candidate.Artifact}
		result = gate.Result{Receipt: receipt, Verdict: spec.Verdict()}
		return o.append(eventGate, receipt)
	})
	return result, err
}

func (o *Operation) Gate(id string) (gate.Result, error) {
	current, err := o.current()
	if err != nil {
		return gate.Result{}, err
	}
	receipt := current.gates[id]
	if receipt.ID == "" {
		return gate.Result{}, errors.New("gate does not exist")
	}
	return gate.Result{Receipt: receipt, Verdict: receipt.Verdict()}, nil
}

func (o *Operation) validateGateComparison(current *state, spec gate.Spec) error {
	if spec.ComparisonID == "" {
		if spec.Verdict() == gate.Pass {
			return errors.New("passing gate requires supported comparison")
		}
		return nil
	}
	observation := current.comparisons[spec.ComparisonID]
	if observation.ID == "" || observation.CandidateID != spec.CandidateID {
		return errors.New("gate comparison does not bind candidate")
	}
	if spec.Verdict() == gate.Pass {
		result, err := o.Comparison(spec.ComparisonID)
		if err != nil || result.Verdict != comparison.SupportedImprovement {
			return errors.New("passing gate requires supported improvement")
		}
	}
	return nil
}

func (o *Operation) validateGateEvidence(ref run.EvidenceRef) error {
	if ref.ExperimentID != o.id {
		return errors.New("gate evidence belongs to another experiment")
	}
	operation, err := run.Open(o.root, o.id, ref.RunID)
	if err != nil {
		return err
	}
	_, err = operation.EvidenceAt(ref)
	return err
}
