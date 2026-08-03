package tool

import "errors"

const (
	PrepareTool = "agentlab_prepare"
	RunTool     = "agentlab_run"
	InspectTool = "agentlab_inspect"
	CompareTool = "agentlab_compare"
)

type Definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

func Definitions() []Definition {
	return []Definition{
		{Name: PrepareTool, Description: "Mutate or inspect one AgentLab preparation.", InputSchema: prepareSchema()},
		{Name: RunTool, Description: "Start, attach, stop, or inspect one experiment-scoped Worker run.", InputSchema: runSchema()},
		{Name: InspectTool, Description: "Read one bounded page from a preparation, experiment, or run ledger.", InputSchema: inspectSchema()},
		{Name: CompareTool, Description: "Record or read an exact-candidate comparison or gate.", InputSchema: compareSchema()},
	}
}

func Projection(provider string) ([]map[string]any, error) {
	definitions := Definitions()
	result := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
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

func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type": "object", "properties": properties, "required": required,
		"additionalProperties": false,
	}
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func enumProperty(values ...string) map[string]any {
	items := make([]any, len(values))
	for index, value := range values {
		items[index] = value
	}
	return map[string]any{"type": "string", "enum": items}
}

func prepareSchema() map[string]any {
	return objectSchema(map[string]any{
		"action":         enumProperty("begin", "record_fact", "propose_decision", "resolve", "assay", "challenge_basis", "challenge", "seal", "status"),
		"root":           stringProperty("Optional AgentLab storage root."),
		"preparation_id": stringProperty("Preparation id for non-request actions."),
		"request_path":   stringProperty("Strict JSON request file for mutation actions."),
	}, "action")
}

func runSchema() map[string]any {
	return objectSchema(map[string]any{
		"action":        enumProperty("start", "attach_begin", "attach_poll", "stop", "status"),
		"root":          stringProperty("Optional AgentLab storage root."),
		"experiment_id": stringProperty("Experiment id."), "run_id": stringProperty("Run id."),
		"adapter":      stringProperty("Attached runtime adapter name."),
		"stream_path":  stringProperty("Adapter-owned stream locator path."),
		"request_path": stringProperty("Strict owned-run request file for start."),
		"first_event":  stringProperty("First-event duration."), "soft_idle": stringProperty("Soft-idle duration."),
		"hard_idle": stringProperty("Hard-idle duration."), "kill_on_hard_idle": map[string]any{"type": "boolean"},
		"reason": stringProperty("Durable stop reason."),
	}, "action", "experiment_id", "run_id")
}

func inspectSchema() map[string]any {
	return objectSchema(map[string]any{
		"root":           stringProperty("Optional AgentLab storage root."),
		"scope":          enumProperty("preparation", "experiment", "run"),
		"preparation_id": stringProperty("Preparation id."), "experiment_id": stringProperty("Experiment id."),
		"run_id": stringProperty("Run id."), "after": map[string]any{"type": "integer", "minimum": 0},
		"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	}, "scope", "after", "limit")
}

func compareSchema() map[string]any {
	return objectSchema(map[string]any{
		"action": enumProperty("record", "show", "gate_record", "gate_show"), "root": stringProperty("Optional AgentLab storage root."),
		"experiment_id": stringProperty("Experiment id for show."),
		"comparison_id": stringProperty("Comparison id for show."),
		"gate_id":       stringProperty("Gate id for gate show."),
		"request_path":  stringProperty("Strict comparison or gate request file for record."),
	}, "action")
}
