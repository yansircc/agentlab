package tool

import (
	"errors"

	"github.com/yansircc/agentlab/internal/experiment"
)

type Definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

func Definitions() []Definition {
	result := make([]Definition, 0, len(ActiveToolNames()))
	for _, name := range ActiveToolNames() {
		switch name {
		case ApplyTool:
			result = append(result, Definition{Name: name, Description: "Submit a closed AgentLab domain operation.", InputSchema: applySchema()})
		case RunTool:
			result = append(result, Definition{Name: name, Description: "Execute one decision-bound AgentLab runtime operation.", InputSchema: runSchema()})
		case InspectTool:
			result = append(result, Definition{Name: name, Description: "Read one bounded public AgentLab projection.", InputSchema: inspectSchema()})
		case CompareTool:
			result = append(result, Definition{Name: name, Description: "Record or read exact-candidate comparison and gate facts.", InputSchema: compareSchema()})
		default:
			panic("unknown AgentLab provider tool")
		}
	}
	return result
}

func Projection(provider string) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(ActiveToolNames()))
	for _, definition := range Definitions() {
		switch provider {
		case "anthropic":
			result = append(result, map[string]any{"name": definition.Name, "description": definition.Description, "input_schema": definition.InputSchema})
		case "openai_responses":
			result = append(result, map[string]any{"type": "function", "name": definition.Name, "description": definition.Description, "parameters": definition.InputSchema})
		default:
			return nil, errors.New("provider must be anthropic or openai_responses")
		}
	}
	return result, nil
}

func object(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func enum(values ...string) map[string]any { return map[string]any{"type": "string", "enum": values} }
func text(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}
func opaque(description string) map[string]any {
	return map[string]any{"type": "object", "description": description}
}

// supervisorDecisionSchema exposes the exact public decision shape so any
// model can construct a valid decision-bound effect without guessing. The
// Go-side Validate remains the authority; the schema only teaches the shape.
func supervisorDecisionSchema() map[string]any {
	return object(map[string]any{
		"id":               text("Write-once decision identity."),
		"worker_run":       text("Worker run id this decision cites."),
		"evidence_through": map[string]any{"type": "integer", "minimum": 0, "description": "Cited evidence sequence; 0 for the fresh bootstrap start."},
		"claim":            text("Public claim the decision records."),
		"action":           enum(string(experiment.DecisionWorkerStart), string(experiment.DecisionCoderStart), string(experiment.DecisionStop), string(experiment.DecisionCheckpoint), string(experiment.DecisionFork), string(experiment.DecisionFinding), string(experiment.DecisionHandoff), string(experiment.DecisionDiagnosis), string(experiment.DecisionCandidate), string(experiment.DecisionIntervention), string(experiment.DecisionRunBinding), string(experiment.DecisionComparison)),
		"evidence": map[string]any{"type": "array", "items": object(map[string]any{
			"experiment_id": text("Experiment id of the cited evidence."),
			"run_id":        text("Worker run id of the cited evidence."),
			"sequence":      map[string]any{"type": "integer", "minimum": 1},
			"item":          map[string]any{"type": "integer", "minimum": 0},
		}, "experiment_id", "run_id", "sequence", "item")},
		"falsifier": text("Public condition that would falsify the claim."),
	}, "id", "worker_run", "claim", "action", "falsifier")
}

func applySchema() map[string]any {
	return object(map[string]any{
		"action":           enum(applyActionNames()...),
		"user_intent":      opaque("Host-issued immutable user-intent artifact ref."),
		"source_snapshot":  opaque("Host-issued immutable source-snapshot artifact ref."),
		"public_artifacts": map[string]any{"type": "array", "items": opaque("Host-issued public artifact ref.")},
		"fact":             opaque("Repository fact."), "decision": supervisorDecisionSchema(), "resolution": opaque("Preparation resolution."), "assay": opaque("Leakage assay."), "challenge": opaque("Challenge."),
		"binding": opaque("Decision-bound run binding."), "origin": opaque("Closed run origin."), "prepared": opaque("Host-issued complete run-input artifact."), "value": opaque("Closed decision-bound domain value."),
		"finding_ids": map[string]any{"type": "array", "items": text("Finding id.")},
	}, "action")
}

func runSchema() map[string]any {
	return object(map[string]any{
		"action":   enum(runActionNames()...),
		"decision": supervisorDecisionSchema(), "effect_id": text("Write-once effect identity."),
		"runtime_ref": text("Host-issued opaque runtime profile ref."), "run_id": text("Experiment-scoped opaque run ref."), "handoff": opaque("Experiment-owned Coder handoff ref."),
		"reason": text("Durable stop reason."), "entry_locator": text("Public runtime-tree entry locator."), "checkpoint": opaque("Runtime checkpoint ref."), "child_run": text("Host-prepared child run id."),
	}, "action")
}

func inspectSchema() map[string]any {
	return object(map[string]any{
		"scope": enum(inspectScopeNames()...), "run_id": text("Experiment-scoped opaque run ref."),
		"after": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	}, "scope", "after", "limit")
}

func compareSchema() map[string]any {
	return object(map[string]any{
		"action": enum(compareActionNames()...), "value": opaque("Closed decision-bound comparison or gate."),
		"comparison_id": text("Experiment-scoped opaque comparison ref."), "gate_id": text("Experiment-scoped opaque gate ref."),
	}, "action")
}
