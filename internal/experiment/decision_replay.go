package experiment

import "github.com/yansircc/agentlab/internal/ledger"

func (s *state) decisionEffect(record ledger.Record) error {
	var value DecisionBoundEffect
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.runs[value.Decision.WorkerRun].RunID == "" || s.runs[value.Intent.RunID].RunID == "" || s.decisions[value.Decision.ID].ID != "" {
		return invalid(record, "invalid decision-bound effect")
	}
	if _, exists := s.effects[value.Intent.ID]; exists {
		return invalid(record, "duplicate decision-bound effect")
	}
	s.effects[value.Intent.ID] = value
	s.effectOrder = append(s.effectOrder, value.Intent.ID)
	s.decisions[value.Decision.ID] = value.Decision
	s.decisionOrder = append(s.decisionOrder, value.Decision.ID)
	return nil
}

func (s *state) decisionFinding(record ledger.Record) error {
	var value DecisionBoundFinding
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.runs[value.Decision.WorkerRun].RunID == "" || s.decisions[value.Decision.ID].ID != "" || s.findings[value.Finding.ID].ID != "" {
		return invalid(record, "invalid decision-bound finding")
	}
	s.findings[value.Finding.ID] = value.Finding
	s.order = append(s.order, value.Finding.ID)
	s.decisions[value.Decision.ID] = value.Decision
	s.decisionOrder = append(s.decisionOrder, value.Decision.ID)
	return nil
}

func (s *state) decisionHandoff(record ledger.Record) error {
	var value DecisionBoundHandoff
	if decode(record.Data, &value) != nil || value.Validate() != nil {
		return invalid(record, "invalid decision-bound handoff")
	}
	_, exists := s.handoffs[value.Handoff.Artifact]
	if s.runs[value.Decision.WorkerRun].RunID == "" || s.decisions[value.Decision.ID].ID != "" || exists {
		return invalid(record, "invalid decision-bound handoff")
	}
	for _, id := range value.Handoff.FindingIDs {
		if s.findings[id].ID == "" {
			return invalid(record, "handoff references absent finding")
		}
	}
	s.handoffs[value.Handoff.Artifact] = value.Handoff
	s.decisions[value.Decision.ID] = value.Decision
	s.decisionOrder = append(s.decisionOrder, value.Decision.ID)
	return nil
}
