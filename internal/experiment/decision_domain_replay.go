package experiment

import (
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/ledger"
)

func (s *state) decisionDiagnosis(record ledger.Record) error {
	var value DecisionBoundDiagnosis
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.runs[value.Decision.WorkerRun].RunID == "" || s.decisions[value.Decision.ID].ID != "" || s.diagnoses[value.Diagnosis.ID].ID != "" || value.Diagnosis.SourceSnapshot != s.begun.Source || !s.hasFindings(value.Diagnosis.FindingIDs) {
		return invalid(record, "invalid decision-bound diagnosis")
	}
	s.diagnoses[value.Diagnosis.ID] = value.Diagnosis
	s.diagnosisOrder = append(s.diagnosisOrder, value.Diagnosis.ID)
	s.addDecision(value.Decision)
	return nil
}

func (s *state) decisionCandidate(record ledger.Record) error {
	var value DecisionBoundCandidate
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.runs[value.Decision.WorkerRun].RunID == "" || s.decisions[value.Decision.ID].ID != "" || s.candidates[value.Candidate.ID].ID != "" {
		return invalid(record, "invalid decision-bound candidate")
	}
	diagnosed := s.diagnoses[value.Candidate.DiagnosisID]
	if diagnosed.ID == "" || diagnosed.State != diagnosis.Established {
		return invalid(record, "candidate has no established diagnosis")
	}
	s.candidates[value.Candidate.ID] = value.Candidate
	s.candidateOrder = append(s.candidateOrder, value.Candidate.ID)
	s.addDecision(value.Decision)
	return nil
}

func (s *state) decisionContinue(record ledger.Record) error {
	var value DecisionBoundContinue
	if decode(record.Data, &value) != nil || value.Validate() != nil || s.runs[value.Decision.WorkerRun].RunID == "" || s.decisions[value.Decision.ID].ID != "" {
		return invalid(record, "invalid decision-bound continue")
	}
	s.addDecision(value.Decision)
	return nil
}

func (s *state) addDecision(value SupervisorDecision) {
	s.decisions[value.ID] = value
	s.decisionOrder = append(s.decisionOrder, value.ID)
}
