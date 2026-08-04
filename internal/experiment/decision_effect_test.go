package experiment

import (
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func TestDecisionBoundEffectRequiresSameWorkerForWorkerEffects(t *testing.T) {
	ref := artifact.Ref{Scope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
	decision := SupervisorDecision{ID: "decision", WorkerRun: "evidence-worker", EvidenceThrough: 1, Claim: "public failure", Action: DecisionStop, Evidence: []run.EvidenceRef{{ExperimentID: "experiment", RunID: "evidence-worker", Sequence: 1}}, Falsifier: "objective success"}
	value := DecisionBoundEffect{Decision: decision, Intent: effect.Intent{ID: "decision", RunID: "target-worker", Kind: effect.Stop, Payload: ref}}
	if value.Validate() == nil {
		t.Fatal("stop decision was allowed to target a different Worker run")
	}
	value.Decision.Action, value.Intent.Kind = DecisionCoderStart, effect.CoderStart
	if value.Validate() != nil {
		t.Fatal("coder start lost its parent-evidence handoff semantics")
	}
}
