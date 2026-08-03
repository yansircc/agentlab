package tool

import "errors"

type compareInput struct {
	Action       string `json:"action"`
	Root         string `json:"root,omitempty"`
	ExperimentID string `json:"experiment_id,omitempty"`
	ComparisonID string `json:"comparison_id,omitempty"`
	GateID       string `json:"gate_id,omitempty"`
	RequestPath  string `json:"request_path,omitempty"`
}

func (input compareInput) invocation() (Invocation, error) {
	switch input.Action {
	case "record", "gate_record":
		if input.RequestPath == "" || input.RequestPath == "-" || input.ExperimentID != "" || input.ComparisonID != "" || input.GateID != "" {
			return Invocation{}, errors.New("compare record requires only request path")
		}
		command := "compare"
		if input.Action == "gate_record" {
			command = "gate"
		}
		args := append([]string{command, "record"}, rootArgs(input.Root)...)
		return Invocation{Args: append(args, "-request", input.RequestPath)}, nil
	case "show":
		if input.RequestPath != "" || input.ExperimentID == "" || input.ComparisonID == "" || input.GateID != "" {
			return Invocation{}, errors.New("compare show requires experiment and comparison ids")
		}
		args := append([]string{"compare", "show"}, rootArgs(input.Root)...)
		return Invocation{Args: append(args, "-experiment", input.ExperimentID, "-comparison", input.ComparisonID)}, nil
	case "gate_show":
		if input.RequestPath != "" || input.ExperimentID == "" || input.ComparisonID != "" || input.GateID == "" {
			return Invocation{}, errors.New("gate show requires experiment and gate ids")
		}
		args := append([]string{"gate", "show"}, rootArgs(input.Root)...)
		return Invocation{Args: append(args, "-experiment", input.ExperimentID, "-gate", input.GateID)}, nil
	default:
		return Invocation{}, errors.New("unknown compare action")
	}
}
