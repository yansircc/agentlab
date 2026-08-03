package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

func (o *Operation) RenderHandoffWithDecision(decision SupervisorDecision, findingIDs []string) (HandoffResult, error) {
	if decision.Action != DecisionHandoff {
		return HandoffResult{}, errors.New("decision-bound handoff is invalid")
	}
	if err := o.validateDecisionEvidence(decision); err != nil {
		return HandoffResult{}, err
	}
	var result HandoffResult
	err := o.mutate(func(current *state) error {
		if current.begun == nil || current.decisions[decision.ID].ID != "" {
			return errors.New("decision-bound handoff identity is invalid")
		}
		value, err := o.renderHandoff(*current, findingIDs)
		if err != nil {
			return err
		}
		record := HandoffRecord{Artifact: value.Artifact, FindingIDs: append([]string(nil), findingIDs...)}
		if record.Validate() != nil {
			return errors.New("handoff record is invalid")
		}
		result = value
		return o.append(eventDecisionHandoff, DecisionBoundHandoff{Decision: decision, Handoff: record})
	})
	return result, err
}

func (o *Operation) Handoff(ref artifact.Ref) (HandoffRecord, error) {
	current, err := o.current()
	if err != nil {
		return HandoffRecord{}, err
	}
	value, exists := current.handoffs[ref]
	if !exists {
		return HandoffRecord{}, errors.New("handoff is not experiment-owned")
	}
	return value, nil
}
