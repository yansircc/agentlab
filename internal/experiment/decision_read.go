package experiment

import "errors"

func (o *Operation) SupervisorDecision(id string) (SupervisorDecision, error) {
	current, err := o.current()
	if err != nil {
		return SupervisorDecision{}, err
	}
	value := current.decisions[id]
	if value.ID == "" {
		return SupervisorDecision{}, errors.New("supervisor decision does not exist")
	}
	return value, nil
}

func (o *Operation) SupervisorDecisions() ([]SupervisorDecision, error) {
	current, err := o.current()
	if err != nil {
		return nil, err
	}
	result := make([]SupervisorDecision, 0, len(current.decisionOrder))
	for _, id := range current.decisionOrder {
		result = append(result, current.decisions[id])
	}
	return result, nil
}

func (o *Operation) GateDecision(id string) (SupervisorDecision, error) {
	current, err := o.current()
	if err != nil {
		return SupervisorDecision{}, err
	}
	value := current.gateDecisions[id]
	if value.ID == "" {
		return SupervisorDecision{}, errors.New("gate has no supervisor decision")
	}
	return value, nil
}
