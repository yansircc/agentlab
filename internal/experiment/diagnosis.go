package experiment

import (
	"bytes"
	"errors"

	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/source"
)

func (o *Operation) RecordDiagnosis(value diagnosis.Diagnosis) error {
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
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
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
		return o.append(eventDiagnosis, value)
	})
}

func (o *Operation) BindCandidate(id, diagnosisID string, candidateBytes []byte) (diagnosis.RepairCandidate, error) {
	artifactRef, err := o.artifacts.Put(candidateBytes)
	if err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	value := diagnosis.RepairCandidate{ID: id, DiagnosisID: diagnosisID, Artifact: artifactRef}
	if err := value.Validate(); err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	err = o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		diagnosed := current.diagnoses[diagnosisID]
		if diagnosed.ID == "" || diagnosed.State != diagnosis.Established {
			return errors.New("repair candidate requires established diagnosis")
		}
		if current.candidates[id].ID != "" {
			return errors.New("repair candidate id already exists")
		}
		return o.append(eventCandidate, value)
	})
	return value, err
}
