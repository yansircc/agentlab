package tool

import (
	"errors"
	"strconv"
)

type inspectInput struct {
	Root          string  `json:"root,omitempty"`
	Scope         string  `json:"scope"`
	PreparationID string  `json:"preparation_id,omitempty"`
	ExperimentID  string  `json:"experiment_id,omitempty"`
	RunID         string  `json:"run_id,omitempty"`
	After         *uint64 `json:"after"`
	Limit         int     `json:"limit"`
}

func (input inspectInput) invocation() (Invocation, error) {
	if input.After == nil || input.Limit < 1 || input.Limit > 1000 {
		return Invocation{}, errors.New("inspect requires after and limit 1..1000")
	}
	args := append([]string{"inspect"}, rootArgs(input.Root)...)
	switch input.Scope {
	case "preparation":
		if input.PreparationID == "" || input.ExperimentID != "" || input.RunID != "" {
			return Invocation{}, errors.New("preparation inspect target is invalid")
		}
		args = append(args, "-preparation", input.PreparationID)
	case "experiment":
		if input.PreparationID != "" || input.ExperimentID == "" || input.RunID != "" {
			return Invocation{}, errors.New("experiment inspect target is invalid")
		}
		args = append(args, "-experiment", input.ExperimentID)
	case "run":
		if input.PreparationID != "" || input.ExperimentID == "" || input.RunID == "" {
			return Invocation{}, errors.New("run inspect target is invalid")
		}
		args = append(args, "-experiment", input.ExperimentID, "-run", input.RunID)
	default:
		return Invocation{}, errors.New("unknown inspect scope")
	}
	args = append(args, "-after", strconv.FormatUint(*input.After, 10), "-limit", strconv.Itoa(input.Limit))
	return Invocation{Args: args}, nil
}
