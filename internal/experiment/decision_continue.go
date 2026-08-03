package experiment

import "errors"

func (o *Operation) RecordContinueWithDecision(value DecisionBoundContinue) error {
	if value.Validate() != nil {
		return errors.New("decision-bound continue is invalid")
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.begun == nil || current.runs[value.Decision.WorkerRun].RunID == "" || current.decisions[value.Decision.ID].ID != "" {
			return errors.New("decision-bound continue identity is invalid")
		}
		return o.append(eventDecisionContinue, value)
	})
}
