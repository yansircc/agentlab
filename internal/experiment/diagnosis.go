package experiment

import (
	"bytes"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/run"
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

// BindCandidate derives a repair candidate only from a terminal Coder receipt.
// A source snapshot alone has no process, session, or workspace provenance.
func (o *Operation) BindCandidate(id, diagnosisID, coderRunID string, completion artifact.Ref) (diagnosis.RepairCandidate, error) {
	return o.bindCandidate(id, diagnosisID, coderRunID, completion, nil)
}

func (o *Operation) bindCandidate(id, diagnosisID, coderRunID string, completionRef artifact.Ref, decision *SupervisorDecision) (diagnosis.RepairCandidate, error) {
	completion, err := o.coderCompletion(coderRunID, completionRef)
	if err != nil {
		return diagnosis.RepairCandidate{}, err
	}
	value := diagnosis.RepairCandidate{ID: id, DiagnosisID: diagnosisID, CoderRun: coderRunID, Completion: completionRef, Artifact: completion.Candidate}
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
		if err := o.validateCandidateCompletion(*current, value); err != nil {
			return err
		}
		if current.candidates[id].ID != "" || (decision != nil && current.decisions[decision.ID].ID != "") {
			return errors.New("repair candidate id already exists")
		}
		if decision == nil {
			return o.append(eventCandidate, value)
		}
		return o.append(eventDecisionCandidate, decisionBoundCandidateRecorded{Decision: *decision, Candidate: value})
	})
	return value, err
}

func (o *Operation) coderCompletion(runID string, receipt artifact.Ref) (run.CoderCompletion, error) {
	completion, err := o.coderTerminalCompletion(runID, receipt)
	if err != nil {
		return run.CoderCompletion{}, err
	}
	current, err := o.current()
	if err != nil {
		return run.CoderCompletion{}, err
	}
	if _, err := o.coderStartForCompletion(current, runID, completion); err != nil {
		return run.CoderCompletion{}, errors.New("candidate completion is not decision-bound")
	}
	return completion, nil
}

func (o *Operation) coderTerminalCompletion(runID string, receipt artifact.Ref) (run.CoderCompletion, error) {
	if !idPattern.MatchString(runID) || !receipt.Valid() {
		return run.CoderCompletion{}, errors.New("coder completion reference is invalid")
	}
	coderRun, err := run.Open(o.root, o.id, runID)
	if err != nil {
		return run.CoderCompletion{}, err
	}
	actual, completion, err := coderRun.CoderCompletionReceipt()
	if err != nil || actual != receipt {
		return run.CoderCompletion{}, errors.New("candidate completion is not owned by Coder run")
	}
	return completion, nil
}

func (o *Operation) validateCandidateCompletion(current state, candidate diagnosis.RepairCandidate) error {
	if current.runs[candidate.CoderRun].RunID == "" {
		return errors.New("candidate Coder run is not experiment-bound")
	}
	completion, err := o.coderTerminalCompletion(candidate.CoderRun, candidate.Completion)
	if err != nil || completion.Candidate != candidate.Artifact || completion.Profile.SourceSnapshot != current.begun.Source || current.handoffs[completion.Profile.Handoff].Artifact != completion.Profile.Handoff {
		return errors.New("candidate completion differs from experiment authority")
	}
	if _, err := o.coderStartForCompletion(current, candidate.CoderRun, completion); err != nil {
		return errors.New("candidate completion differs from experiment authority")
	}
	return nil
}
