package experiment

import (
	"bytes"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/source"
)

func (o *Operation) RecordDiagnosis(value diagnosis.Diagnosis) error {
	if err := o.validateDiagnosis(value); err != nil {
		return err
	}
	return o.recordDiagnosis(value, nil)
}

func (o *Operation) validateDiagnosis(value diagnosis.Diagnosis) error {
	if err := value.Validate(); err != nil {
		return err
	}
	snapshot, err := source.Load(o.artifacts, value.SourceSnapshot)
	if err != nil {
		return err
	}
	for _, ref := range value.SourceEvidence {
		if !snapshot.Contains(ref.Path, ref.Artifact) {
			return errors.New("source evidence is not a member of exact source snapshot")
		}
		data, err := o.artifacts.Read(ref.Artifact)
		if err != nil {
			return err
		}
		lines := bytes.Count(data, []byte{'\n'})
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lines++
		}
		if ref.EndLine > lines {
			return errors.New("source evidence line range exceeds exact artifact")
		}
	}
	return nil
}

func (o *Operation) recordDiagnosis(value diagnosis.Diagnosis, decision *SupervisorDecision) error {
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		if decision != nil && current.decisions[decision.ID].ID != "" {
			return errors.New("decision identity already exists")
		}
		if value.SourceSnapshot != current.begun.Source {
			return errors.New("diagnosis source snapshot differs from experiment")
		}
		if !current.hasFindings(value.FindingIDs) {
			return errors.New("diagnosis references absent finding")
		}
		if current.diagnoses[value.ID].ID != "" {
			return errors.New("diagnosis id already exists")
		}
		if decision == nil {
			return o.append(eventDiagnosis, value)
		}
		return o.append(eventDecisionDiagnosis, DecisionBoundDiagnosis{Decision: *decision, Diagnosis: value})
	})
}

// BindCandidate accepts only a sealed source snapshot. Raw bytes are never a
// repair candidate because a Worker executes a build of the source tree.
func (o *Operation) BindCandidate(id, diagnosisID string, sourceSnapshot artifact.Ref) (diagnosis.RepairCandidate, error) {
	return o.bindCandidate(id, diagnosisID, sourceSnapshot, nil)
}

func (o *Operation) bindCandidate(id, diagnosisID string, sourceSnapshot artifact.Ref, decision *SupervisorDecision) (diagnosis.RepairCandidate, error) {
	if _, err := source.Load(o.artifacts, sourceSnapshot); err != nil {
		return diagnosis.RepairCandidate{}, errors.New("repair candidate source snapshot is invalid")
	}
	value := diagnosis.RepairCandidate{ID: id, DiagnosisID: diagnosisID, Artifact: sourceSnapshot}
	if err := value.Validate(); err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	err := o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		diagnosed := current.diagnoses[diagnosisID]
		if diagnosed.ID == "" || diagnosed.State != diagnosis.Established {
			return errors.New("repair candidate requires established diagnosis")
		}
		if current.candidates[id].ID != "" || (decision != nil && current.decisions[decision.ID].ID != "") {
			return errors.New("repair candidate id already exists")
		}
		if decision == nil {
			return o.append(eventCandidate, value)
		}
		return o.append(eventDecisionCandidate, DecisionBoundCandidate{Decision: *decision, Candidate: value})
	})
	return value, err
}
