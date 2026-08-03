package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func (o *Operation) CommitDecisionBoundEffect(value DecisionBoundEffect) error {
	if value.Validate() != nil {
		return errors.New("decision-bound effect is invalid")
	}
	if _, err := o.artifacts.Read(value.Intent.Payload); err != nil {
		return err
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		if current.runs[value.Decision.WorkerRun].RunID == "" || current.runs[value.Intent.RunID].RunID == "" {
			return errors.New("decision-bound effect targets an unbound run")
		}
		if _, exists := current.effects[value.Intent.ID]; exists {
			return errors.New("decision-bound effect id already exists")
		}
		if value.Intent.Kind == effect.CoderStart {
			target, err := run.Open(o.root, o.id, value.Intent.RunID)
			if err != nil {
				return err
			}
			profile, err := target.CoderProfile(value.Intent)
			if err != nil || current.handoffs[profile.Handoff].Artifact != profile.Handoff || profile.SourceSnapshot != current.begun.Source {
				return errors.New("coder start requires an experiment-owned handoff")
			}
		}
		return o.append(eventDecisionEffect, value)
	})
}

func (o *Operation) DecisionBoundEffect(id string) (DecisionBoundEffect, error) {
	current, err := o.current()
	if err != nil {
		return DecisionBoundEffect{}, err
	}
	value, exists := current.effects[id]
	if !exists {
		return DecisionBoundEffect{}, errors.New("decision-bound effect does not exist")
	}
	return value, nil
}

func (o *Operation) validateDecisionEvidence(value SupervisorDecision) error {
	if value.Validate() != nil {
		return errors.New("supervisor decision is invalid")
	}
	for _, ref := range value.Evidence {
		if ref.ExperimentID != o.id || ref.RunID != value.WorkerRun || ref.Sequence > value.EvidenceThrough {
			return errors.New("supervisor decision evidence exceeds its public prefix")
		}
		operation, err := run.Open(o.root, o.id, value.WorkerRun)
		if err != nil {
			return err
		}
		if _, err := operation.EvidenceAt(ref); err != nil {
			return err
		}
	}
	return nil
}

func (o *Operation) SettleDecisionBoundEffect(id string, evidence []byte) (effect.Receipt, error) {
	value, err := o.DecisionBoundEffect(id)
	if err != nil {
		return effect.Receipt{}, err
	}
	operation, err := run.Open(o.root, o.id, value.Intent.RunID)
	if err != nil {
		return effect.Receipt{}, err
	}
	return operation.SettleEffect(value.Intent, evidence)
}
