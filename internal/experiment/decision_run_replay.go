package experiment

import "github.com/yansircc/agentlab/internal/ledger"

func (s *state) decisionRun(record ledger.Record) error {
	var value decisionRunRecord
	if decode(record.Data, &value) != nil || value.Decision.Validate() != nil || value.Decision.Action != DecisionRunBinding || !idPattern.MatchString(value.Binding.RunID) || !validRef(value.Binding.Manifest) || s.decisions[value.Decision.ID].ID != "" || s.runs[value.Binding.RunID].RunID != "" {
		return invalid(record, "invalid decision-bound run binding")
	}
	s.runs[value.Binding.RunID] = value.Binding
	s.runOrder = append(s.runOrder, value.Binding.RunID)
	s.addDecision(value.Decision)
	return nil
}
