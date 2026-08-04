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
	DecisionWorkerStart  DecisionAction = "worker_start"
	DecisionCoderStart   DecisionAction = "coder_start"
	DecisionStop         DecisionAction = "stop"
	DecisionCheckpoint   DecisionAction = "checkpoint"
	DecisionFork         DecisionAction = "fork"
	DecisionFinding      DecisionAction = "finding"
	DecisionHandoff      DecisionAction = "coder_handoff"
	DecisionDiagnosis    DecisionAction = "diagnosis"
	DecisionCandidate    DecisionAction = "candidate"
	DecisionIntervention DecisionAction = "intervention"
	DecisionRunBinding   DecisionAction = "run_binding"
	DecisionComparison   DecisionAction = "comparison"
	DecisionGate         DecisionAction = "gate"
	DecisionContinue     DecisionAction = "continue"
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

// DecisionBoundContinue makes an otherwise effect-free material decision
// durable without inventing a second ledger owner.
type DecisionBoundContinue struct {
	Decision SupervisorDecision `json:"decision"`
}

func (value SupervisorDecision) Validate() error {
	if !decisionIDPattern.MatchString(value.ID) || !idPattern.MatchString(value.WorkerRun) || value.Claim == "" || len(value.Claim) > 4096 || value.Falsifier == "" || len(value.Falsifier) > 4096 || len(value.Evidence) > 100 || !value.Action.valid() {
		return errors.New("supervisor decision is invalid")
	}
	if value.isBootstrapWorkerStart() {
		return nil
	}
	if value.EvidenceThrough == 0 || len(value.Evidence) == 0 {
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

// isBootstrapWorkerStart is the one evidence-free decision: it can launch an
// unstarted FreshOrigin run that is already bound to sealed preparation. Once
// the Worker has public evidence, every decision uses the normal prefix rule.
func (value SupervisorDecision) isBootstrapWorkerStart() bool {
	return value.Action == DecisionWorkerStart && value.EvidenceThrough == 0 && len(value.Evidence) == 0
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

func (value DecisionBoundContinue) Validate() error {
	if value.Decision.Validate() != nil || value.Decision.Action != DecisionContinue {
		return errors.New("decision-bound continue is invalid")
	}
	return nil
}

func (value DecisionAction) valid() bool {
	return value.effectKind() != "" || value == DecisionFinding || value == DecisionHandoff || value == DecisionDiagnosis || value == DecisionCandidate || value == DecisionIntervention || value == DecisionRunBinding || value == DecisionComparison || value == DecisionGate || value == DecisionContinue
}

func (value DecisionBoundEffect) Validate() error {
	if value.Decision.Validate() != nil || value.Intent.Validate() != nil || value.Decision.ID != value.Intent.ID || value.Decision.Action.effectKind() != value.Intent.Kind || (value.Decision.isBootstrapWorkerStart() && value.Decision.WorkerRun != value.Intent.RunID) || value.Decision.Action.requiresSameWorkerRun() && value.Decision.WorkerRun != value.Intent.RunID {
		return errors.New("decision-bound effect is invalid")
	}
	return nil
}

func (value DecisionAction) requiresSameWorkerRun() bool {
	return value == DecisionStop || value == DecisionCheckpoint || value == DecisionFork
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
