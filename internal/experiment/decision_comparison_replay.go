package experiment

import "github.com/yansircc/agentlab/internal/ledger"

func (s *state) decisionComparison(record ledger.Record) error {
	var value DecisionBoundComparison
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.decisions[value.Decision.ID].ID != "" || s.comparisons[value.Observation.ID].ID != "" || s.runs[value.Decision.WorkerRun].RunID == "" || s.candidates[value.Observation.CandidateID].ID == "" || !s.hasRuns(value.Observation.BaselineRuns) || !s.hasRuns(value.Observation.CandidateRuns) || !diagnosisOwnsClaims(s.diagnoses[s.candidates[value.Observation.CandidateID].DiagnosisID], value.Observation.Policy.RequiredClaims) {
		return invalid(record, "invalid decision-bound comparison")
	}
	s.comparisons[value.Observation.ID] = value.Observation
	s.comparisonOrder = append(s.comparisonOrder, value.Observation.ID)
	s.addDecision(value.Decision)
	return nil
}

func (s *state) decisionGate(record ledger.Record) error {
	var value DecisionBoundGate
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.decisions[value.Decision.ID].ID != "" || s.runs[value.Decision.WorkerRun].RunID == "" || s.gates[value.Receipt.ID].ID != "" {
		return invalid(record, "invalid decision-bound gate")
	}
	candidate := s.candidates[value.Receipt.CandidateID]
	if candidate.ID == "" || value.Receipt.Candidate != candidate.Artifact || !s.gateComparisonMatches(value.Receipt) {
		return invalid(record, "decision gate candidate is invalid")
	}
	for _, blocker := range value.Receipt.BlockerFindings() {
		if s.findings[blocker.ID].ID != "" || !s.hasRunsForEvidence(blocker.Evidence) {
			return invalid(record, "decision gate blocker is invalid")
		}
		s.findings[blocker.ID] = blocker
		s.order = append(s.order, blocker.ID)
	}
	s.gates[value.Receipt.ID] = value.Receipt
	s.gateDecisions[value.Receipt.ID] = value.Decision
	s.gateOrder = append(s.gateOrder, value.Receipt.ID)
	s.addDecision(value.Decision)
	return nil
}
