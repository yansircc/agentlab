package tool

import "errors"

type Definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

func Definitions() []Definition {
	return []Definition{
		{Name: ApplyTool, Description: "Submit a closed AgentLab domain operation.", InputSchema: applySchema()},
		{Name: RunTool, Description: "Execute one decision-bound AgentLab runtime operation.", InputSchema: runSchema()},
		{Name: InspectTool, Description: "Read one bounded public AgentLab projection.", InputSchema: inspectSchema()},
		{Name: CompareTool, Description: "Record or read exact-candidate comparison and gate facts.", InputSchema: compareSchema()},
	}
}

func Projection(provider string) ([]map[string]any, error) {
	result := make([]map[string]any, 0, 4)
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

func applySchema() map[string]any {
	return object(map[string]any{
		"action":           enum("begin_preparation", "record_fact", "propose_preparation_decision", "resolve_preparation_decision", "record_leakage_assay", "challenge_basis", "challenge", "seal_preparation", "begin_experiment", "bind_run", "record_finding", "render_handoff", "record_diagnosis", "bind_candidate", "continue"),
		"user_intent":      opaque("Host-issued immutable user-intent artifact ref."),
		"source_snapshot":  opaque("Host-issued immutable source-snapshot artifact ref."),
		"public_artifacts": map[string]any{"type": "array", "items": opaque("Host-issued public artifact ref.")},
		"fact":             opaque("Repository fact."), "decision": opaque("Supervisor decision."), "resolution": opaque("Preparation resolution."), "assay": opaque("Leakage assay."), "challenge": opaque("Challenge."),
		"binding": opaque("Decision-bound run binding."), "origin": opaque("Closed run origin."), "inputs": opaque("Run inputs of immutable artifact refs."), "value": opaque("Closed decision-bound domain value."),
		"finding_ids": map[string]any{"type": "array", "items": text("Finding id.")},
	}, "action")
}

func runSchema() map[string]any {
	return object(map[string]any{
		"action": enum("start", "poll", "stop", "checkpoint", "fork", "status"),
		"effect": opaque("Closed decision-bound effect."), "runtime_ref": text("Host-issued opaque runtime profile ref."), "run_id": text("Experiment-scoped opaque run ref."),
	}, "action")
}

func inspectSchema() map[string]any {
	return object(map[string]any{
		"scope": enum("preparation", "experiment", "run"), "run_id": text("Experiment-scoped opaque run ref."),
		"after": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	}, "scope", "after", "limit")
}

func compareSchema() map[string]any {
	return object(map[string]any{
		"action": enum("record", "show", "gate_record", "gate_show"), "value": opaque("Closed decision-bound comparison or gate."),
		"comparison_id": text("Experiment-scoped opaque comparison ref."), "gate_id": text("Experiment-scoped opaque gate ref."),
	}, "action")
}
