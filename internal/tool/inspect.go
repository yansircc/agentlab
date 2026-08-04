package tool

import (
	"errors"

	"github.com/yansircc/agentlab/internal/run"
)

type inspectOperation struct {
	Scope string  `json:"scope"`
	RunID string  `json:"run_id,omitempty"`
	After *uint64 `json:"after"`
	Limit int     `json:"limit"`
}

func decodeInspect(data []byte) (Operation, error) {
	var value inspectOperation
	if err := strictDecode(data, &value); err != nil {
		return nil, err
	}
	if value.After == nil || value.Limit < 1 || value.Limit > 1000 {
		return nil, errors.New("inspect page is invalid")
	}
	switch value.Scope {
	case "preparation", "experiment":
		if value.RunID != "" {
			return nil, errors.New("inspect scope carries a run id")
		}
	case "run", "runtime_tree":
		if value.RunID == "" {
			return nil, errors.New("run inspect requires a run id")
		}
	default:
		return nil, errors.New("inspect scope is invalid")
	}
	return value, nil
}

func (inspectOperation) toolName() string { return InspectTool }

func (value inspectOperation) execute(binding Binding) (any, error) {
	switch value.Scope {
	case "preparation":
		op, err := binding.preparation()
		if err != nil {
			return nil, err
		}
		return op.Inspect(*value.After, value.Limit)
	case "experiment":
		op, err := binding.experiment()
		if err != nil {
			return nil, err
		}
		return op.Inspect(*value.After, value.Limit)
	case "run":
		op, err := run.Open(binding.Root, binding.ExperimentID, value.RunID)
		if err != nil {
			return nil, err
		}
		return op.Inspect(*value.After, value.Limit)
	case "runtime_tree":
		if binding.Runtime == nil {
			return nil, errors.New("runtime tree inspection is unavailable")
		}
		return binding.Runtime.RuntimeTree(binding, value.RunID, *value.After, value.Limit)
	default:
		return nil, errors.New("inspect scope is invalid")
	}
}
