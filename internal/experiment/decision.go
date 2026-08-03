package experiment

import (
	"errors"
	"regexp"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/run"
)

var decisionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type DecisionAction string

const (
	DecisionWorkerStart DecisionAction = "worker_start"
	DecisionCoderStart  DecisionAction = "coder_start"
	DecisionStop        DecisionAction = "stop"
	DecisionCheckpoint  DecisionAction = "checkpoint"
	DecisionFork        DecisionAction = "fork"
	DecisionFinding     DecisionAction = "finding"
	DecisionHandoff     DecisionAction = "coder_handoff"
)

// SupervisorDecision is public evidence only. Its timestamp is the enclosing
// experiment-ledger record, so it cannot be authored independently of an intent.
type SupervisorDecision struct {
	ID              string            `json:"id"`
	WorkerRun       string            `json:"worker_run"`
	EvidenceThrough uint64            `json:"evidence_through"`
	Claim           string            `json:"claim"`
	Action          DecisionAction    `json:"action"`
	Evidence        []run.EvidenceRef `json:"evidence"`
	Falsifier       string            `json:"falsifier"`
}

type DecisionBoundEffect struct {
	Decision SupervisorDecision `json:"decision"`
	Intent   effect.Intent      `json:"intent"`
}

type DecisionBoundFinding struct {
	Decision SupervisorDecision `json:"decision"`
	Finding  finding.Finding    `json:"finding"`
}

type DecisionBoundHandoff struct {
	Decision SupervisorDecision `json:"decision"`
	Handoff  HandoffRecord      `json:"handoff"`
}

func (value SupervisorDecision) Validate() error {
	if !decisionIDPattern.MatchString(value.ID) || !idPattern.MatchString(value.WorkerRun) || value.EvidenceThrough == 0 || value.Claim == "" || len(value.Claim) > 4096 || value.Falsifier == "" || len(value.Falsifier) > 4096 || len(value.Evidence) == 0 || len(value.Evidence) > 100 || !value.Action.valid() {
		return errors.New("supervisor decision is invalid")
	}
	seen := map[run.EvidenceRef]bool{}
	for _, ref := range value.Evidence {
		if ref.ExperimentID == "" || ref.RunID == "" || ref.Sequence == 0 || ref.Item < 0 || seen[ref] {
			return errors.New("supervisor decision evidence is invalid")
		}
		seen[ref] = true
	}
	return nil
}

func (value DecisionBoundFinding) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionFinding || value.Finding.Validate() != nil {
		return errors.New("decision-bound finding is invalid")
	}
	return nil
}

func (value DecisionBoundHandoff) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionHandoff || value.Handoff.Validate() != nil {
		return errors.New("decision-bound handoff is invalid")
	}
	return nil
}

func (value DecisionAction) valid() bool {
	return value.effectKind() != "" || value == DecisionFinding || value == DecisionHandoff
}

func (value DecisionBoundEffect) Validate() error {
	if value.Decision.Validate() != nil || value.Intent.Validate() != nil || value.Decision.ID != value.Intent.ID || value.Decision.Action.effectKind() != value.Intent.Kind {
		return errors.New("decision-bound effect is invalid")
	}
	return nil
}

func (value DecisionAction) effectKind() effect.Kind {
	switch value {
	case DecisionWorkerStart:
		return effect.WorkerStart
	case DecisionCoderStart:
		return effect.CoderStart
	case DecisionStop:
		return effect.Stop
	case DecisionCheckpoint:
		return effect.Checkpoint
	case DecisionFork:
		return effect.Fork
	default:
		return ""
	}
}
