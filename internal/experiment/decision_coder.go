package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

// CoderStartForCompletion verifies that a terminal Coder completion came from
// exactly one decision-bound, settled Coder start in this experiment.
func (o *Operation) CoderStartForCompletion(runID string, completion run.CoderCompletion) (DecisionBoundEffect, error) {
	if !idPattern.MatchString(runID) || completion.Validate() != nil {
		return DecisionBoundEffect{}, errors.New("coder completion is invalid")
	}
	current, err := o.current()
	if err != nil {
		return DecisionBoundEffect{}, err
	}
	return o.coderStartForCompletion(current, runID, completion)
}

func (o *Operation) coderStartForCompletion(current state, runID string, completion run.CoderCompletion) (DecisionBoundEffect, error) {
	var result *DecisionBoundEffect
	for _, id := range current.effectOrder {
		value := current.effects[id]
		if value.Intent.RunID != runID || value.Intent.Kind != effect.CoderStart {
			continue
		}
		if result != nil {
			return DecisionBoundEffect{}, errors.New("coder run has multiple start effects")
		}
		copy := value
		result = &copy
	}
	if result == nil {
		return DecisionBoundEffect{}, errors.New("coder completion has no decision-bound start")
	}
	coder, err := run.Open(o.root, o.id, runID)
	if err != nil {
		return DecisionBoundEffect{}, err
	}
	profile, err := coder.CoderProfile(result.Intent)
	if err != nil || profile != completion.Profile {
		return DecisionBoundEffect{}, errors.New("coder completion differs from start effect")
	}
	receipt, exists, err := coder.EffectReceipt(result.Intent.ID)
	if err != nil || !exists || receipt.Kind != effect.CoderStart {
		return DecisionBoundEffect{}, errors.New("coder start effect is not settled")
	}
	return *result, nil
}

func (o *Operation) validateDecisionEffects(current state) error {
	for _, id := range current.effectOrder {
		value := current.effects[id]
		if value.Intent.Kind != effect.CoderStart {
			continue
		}
		coder, err := run.Open(o.root, o.id, value.Intent.RunID)
		if err != nil {
			return err
		}
		profile, err := coder.CoderProfile(value.Intent)
		if err != nil || current.handoffs[profile.Handoff].Artifact != profile.Handoff || current.begun == nil || profile.SourceSnapshot != current.begun.Source {
			return errors.New("coder start effect has no experiment-owned profile")
		}
	}
	return nil
}
