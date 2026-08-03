package experiment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/run"
)

func initialState() state {
	return state{
		findings: map[string]finding.Finding{}, dispositions: map[string]finding.Disposition{},
		diagnoses: map[string]diagnosis.Diagnosis{}, candidates: map[string]diagnosis.RepairCandidate{},
		runs:        map[string]runBinding{},
		comparisons: map[string]comparison.Observation{},
		gates:       map[string]gate.Receipt{},
		effects:     map[string]DecisionBoundEffect{},
		decisions:   map[string]SupervisorDecision{},
		handoffs:    map[artifact.Ref]HandoffRecord{},
	}
}

func (o *Operation) current() (state, error) {
	current := initialState()
	err := o.ledger.Visit(func(record ledger.Record) error {
		if err := current.apply(record); err != nil {
			return err
		}
		current.eventCount = record.Sequence
		return nil
	})
	if err == nil {
		err = o.validateRunLineage(current)
	}
	if err == nil {
		for _, id := range current.decisionOrder {
			if err = o.validateDecisionEvidence(current.decisions[id]); err != nil {
				break
			}
		}
	}
	return current, err
}

func (s *state) apply(record ledger.Record) error {
	if record.Kind != eventBegun && s.begun == nil {
		return fmt.Errorf("experiment event before begin at sequence %d", record.Sequence)
	}
	switch record.Kind {
	case eventBegun:
		var value begun
		if record.Sequence != 1 || s.begun != nil || decode(record.Data, &value) != nil || !idPattern.MatchString(value.PreparationID) || !validRef(value.WorkerInput) || !validRef(value.Source) {
			return invalid(record, "invalid experiment begin")
		}
		s.begun = &value
	case eventFinding:
		var value finding.Finding
		if decode(record.Data, &value) != nil || value.Validate() != nil || s.findings[value.ID].ID != "" {
			return invalid(record, "invalid or duplicate finding")
		}
		s.findings[value.ID] = value
		s.order = append(s.order, value.ID)
	case eventDisposition:
		var value finding.Disposition
		if decode(record.Data, &value) != nil || value.Validate() != nil || s.findings[value.FindingID].ID == "" || s.dispositions[value.FindingID].FindingID != "" {
			return invalid(record, "invalid finding disposition")
		}
		s.dispositions[value.FindingID] = value
	case eventDiagnosis:
		var value diagnosis.Diagnosis
		if decode(record.Data, &value) != nil || value.Validate() != nil || s.diagnoses[value.ID].ID != "" || !s.hasFindings(value.FindingIDs) || value.SourceSnapshot != s.begun.Source {
			return invalid(record, "invalid diagnosis")
		}
		s.diagnoses[value.ID] = value
		s.diagnosisOrder = append(s.diagnosisOrder, value.ID)
	case eventCandidate:
		var value diagnosis.RepairCandidate
		if decode(record.Data, &value) != nil {
			return invalid(record, "invalid repair candidate")
		}
		diagnosed := s.diagnoses[value.DiagnosisID]
		if value.Validate() != nil || s.candidates[value.ID].ID != "" || diagnosed.ID == "" || diagnosed.State != diagnosis.Established {
			return invalid(record, "invalid repair candidate")
		}
		s.candidates[value.ID] = value
		s.candidateOrder = append(s.candidateOrder, value.ID)
	case eventRunBound:
		var value runBinding
		if decode(record.Data, &value) != nil || !idPattern.MatchString(value.RunID) || !validRef(value.Manifest) || s.runs[value.RunID].RunID != "" {
			return invalid(record, "invalid run manifest binding")
		}
		s.runs[value.RunID] = value
		s.runOrder = append(s.runOrder, value.RunID)
	case eventComparison:
		var value comparison.Observation
		if decode(record.Data, &value) != nil || value.Validate() != nil || s.comparisons[value.ID].ID != "" || s.candidates[value.CandidateID].ID == "" || !s.hasRuns(value.BaselineRuns) || !s.hasRuns(value.CandidateRuns) || !diagnosisOwnsClaims(s.diagnoses[s.candidates[value.CandidateID].DiagnosisID], value.Policy.RequiredClaims) {
			return invalid(record, "invalid comparison observation")
		}
		s.comparisons[value.ID] = value
		s.comparisonOrder = append(s.comparisonOrder, value.ID)
	case eventGate:
		var value gate.Receipt
		if decode(record.Data, &value) != nil {
			return invalid(record, "invalid candidate gate")
		}
		candidate := s.candidates[value.CandidateID]
		if value.Validate() != nil || s.gates[value.ID].ID != "" || candidate.ID == "" || value.Candidate != candidate.Artifact || !s.gateComparisonMatches(value) {
			return invalid(record, "invalid candidate gate")
		}
		for _, blocker := range value.BlockerFindings() {
			if s.findings[blocker.ID].ID != "" || !s.hasRunsForEvidence(blocker.Evidence) {
				return invalid(record, "invalid gate blocker finding")
			}
			s.findings[blocker.ID] = blocker
			s.order = append(s.order, blocker.ID)
		}
		s.gates[value.ID] = value
		s.gateOrder = append(s.gateOrder, value.ID)
	case eventDecisionEffect:
		return s.decisionEffect(record)
	case eventDecisionFinding:
		return s.decisionFinding(record)
	case eventDecisionHandoff:
		return s.decisionHandoff(record)
	default:
		return fmt.Errorf("unknown experiment event %q", record.Kind)
	}
	return nil
}

func (s *state) gateComparisonMatches(value gate.Receipt) bool {
	if value.ComparisonID == "" {
		return value.Verdict() == gate.Block
	}
	comparisonValue := s.comparisons[value.ComparisonID]
	return comparisonValue.ID != "" && comparisonValue.CandidateID == value.CandidateID
}

func (s *state) hasRunsForEvidence(refs []run.EvidenceRef) bool {
	for _, ref := range refs {
		if s.runs[ref.RunID].RunID == "" {
			return false
		}
	}
	return true
}

func (s *state) hasRuns(ids []string) bool {
	for _, id := range ids {
		if s.runs[id].RunID == "" {
			return false
		}
	}
	return true
}

func (s *state) hasFindings(ids []string) bool {
	for _, id := range ids {
		if s.findings[id].ID == "" {
			return false
		}
	}
	return true
}

func decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("event data has trailing input")
	}
	return nil
}

func invalid(record ledger.Record, reason string) error {
	return fmt.Errorf("%s at sequence %d", reason, record.Sequence)
}
