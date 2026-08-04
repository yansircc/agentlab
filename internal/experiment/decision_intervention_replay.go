package experiment

import "github.com/yansircc/agentlab/internal/ledger"

func (s *state) decisionIntervention(record ledger.Record) error {
	var value decisionInterventionRecorded
	if decode(record.Data, &value) != nil || value.Decision.Validate() != nil || value.Decision.Action != DecisionIntervention || value.Intervention.Validate() != nil || !validRef(value.Artifact) || s.runs[value.Decision.WorkerRun].RunID == "" || s.decisions[value.Decision.ID].ID != "" || s.interventions[value.Artifact].Contract != "" {
		return invalid(record, "invalid decision-bound intervention")
	}
	s.interventions[value.Artifact] = value.Intervention
	s.interventionOwner[value.Artifact] = value.Decision.ID
	s.interventionOrder = append(s.interventionOrder, value.Artifact)
	s.addDecision(value.Decision)
	return nil
}
