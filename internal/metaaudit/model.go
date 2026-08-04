// Package metaaudit owns Codex-only findings in a root disjoint from the
// evaluated AgentLab root.
package metaaudit

import (
	"errors"
	"regexp"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
)

const Contract = "agentlab.meta-audit.v1"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Trial struct {
	Contract       string       `json:"contract"`
	ExperimentID   string       `json:"experiment_id"`
	EvaluatedScope string       `json:"evaluated_scope"`
	GroundTruth    artifact.Ref `json:"ground_truth"`
}

type Finding struct {
	ID              string            `json:"id"`
	DecisionID      string            `json:"decision_id"`
	WorkerRun       string            `json:"worker_run"`
	EvidenceThrough uint64            `json:"evidence_through"`
	WorkerEvidence  []run.EvidenceRef `json:"worker_evidence"`
	Claim           string            `json:"claim"`
	Falsifier       string            `json:"falsifier"`
	GroundTruth     artifact.Ref      `json:"ground_truth"`
}

// Review records Codex's no-finding assessment of exactly one Supervisor
// decision. It is separate from Finding so a sealed empty finding set cannot
// masquerade as complete Meta-audit coverage.
type Review struct {
	ID              string            `json:"id"`
	DecisionID      string            `json:"decision_id"`
	WorkerRun       string            `json:"worker_run"`
	EvidenceThrough uint64            `json:"evidence_through"`
	WorkerEvidence  []run.EvidenceRef `json:"worker_evidence"`
	Claim           string            `json:"claim"`
	Falsifier       string            `json:"falsifier"`
	GroundTruth     artifact.Ref      `json:"ground_truth"`
}

func (value Trial) Validate() error {
	if value.Contract != Contract || !idPattern.MatchString(value.ExperimentID) || !digest(value.EvaluatedScope) || !value.GroundTruth.Valid() {
		return errors.New("meta-audit trial is invalid")
	}
	return nil
}

func (value Finding) Validate() error {
	if !idPattern.MatchString(value.ID) || !idPattern.MatchString(value.DecisionID) || !idPattern.MatchString(value.WorkerRun) || value.EvidenceThrough == 0 || len(value.WorkerEvidence) == 0 || len(value.WorkerEvidence) > 100 || value.Claim == "" || len(value.Claim) > 4096 || value.Falsifier == "" || len(value.Falsifier) > 4096 || !value.GroundTruth.Valid() {
		return errors.New("meta-audit finding is invalid")
	}
	seen := map[run.EvidenceRef]bool{}
	for _, ref := range value.WorkerEvidence {
		if ref.ExperimentID == "" || ref.RunID == "" || ref.Sequence == 0 || ref.Item < 0 || seen[ref] {
			return errors.New("meta-audit evidence is invalid")
		}
		seen[ref] = true
	}
	return nil
}

func (value Review) Validate() error {
	if !idPattern.MatchString(value.ID) || !idPattern.MatchString(value.DecisionID) || !idPattern.MatchString(value.WorkerRun) || len(value.WorkerEvidence) > 100 || value.Claim == "" || len(value.Claim) > 4096 || value.Falsifier == "" || len(value.Falsifier) > 4096 || !value.GroundTruth.Valid() {
		return errors.New("meta-audit review is invalid")
	}
	if value.EvidenceThrough == 0 {
		if len(value.WorkerEvidence) != 0 {
			return errors.New("meta-audit bootstrap review has evidence")
		}
		return nil
	}
	if len(value.WorkerEvidence) == 0 {
		return errors.New("meta-audit review evidence is absent")
	}
	seen := map[run.EvidenceRef]bool{}
	for _, ref := range value.WorkerEvidence {
		if ref.ExperimentID == "" || ref.RunID == "" || ref.Sequence == 0 || ref.Item < 0 || seen[ref] {
			return errors.New("meta-audit review evidence is invalid")
		}
		seen[ref] = true
	}
	return nil
}

func digest(value string) bool {
	return len(value) == 64 && regexp.MustCompile(`^[a-f0-9]+$`).MatchString(value)
}
