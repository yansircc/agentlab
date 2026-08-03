package tool

import (
	"errors"
	"reflect"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
)

type runOperation interface {
	Operation
	runOperation()
}

func decodeRun(data []byte) (Operation, error) {
	action, err := decodeAction(data)
	if err != nil {
		return nil, err
	}
	switch action {
	case "start", "checkpoint", "fork":
		var value runtimeEffect
		return decodeRunValue(data, &value)
	case "stop":
		var value stopRun
		return decodeRunValue(data, &value)
	case "poll":
		var value pollRun
		return decodeRunValue(data, &value)
	case "status":
		var value statusRun
		return decodeRunValue(data, &value)
	default:
		return nil, errors.New("unknown run action")
	}
}

func decodeRunValue(data []byte, value runOperation) (Operation, error) {
	if err := strictDecode(data, value); err != nil {
		return nil, err
	}
	return value, nil
}

type runtimeEffect struct {
	Action     string                         `json:"action"`
	Effect     experiment.DecisionBoundEffect `json:"effect"`
	RuntimeRef string                         `json:"runtime_ref"`
}

func (runtimeEffect) toolName() string { return RunTool }
func (runtimeEffect) runOperation()    {}

func (value runtimeEffect) execute(binding Binding) (any, error) {
	if value.RuntimeRef == "" || binding.Runtime == nil {
		return nil, errors.New("tool runtime profile is unavailable")
	}
	kind := value.Effect.Intent.Kind
	if (value.Action == "start" && kind != effect.WorkerStart && kind != effect.CoderStart) || (value.Action == "checkpoint" && kind != effect.Checkpoint) || (value.Action == "fork" && kind != effect.Fork) {
		return nil, errors.New("run action and effect kind differ")
	}
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	if err := commitEffect(op, value.Effect); err != nil {
		return nil, err
	}
	switch value.Action {
	case "start":
		return binding.Runtime.Start(binding, value.Effect.Intent, value.RuntimeRef)
	case "checkpoint":
		return binding.Runtime.Checkpoint(binding, value.Effect.Intent, value.RuntimeRef)
	case "fork":
		return binding.Runtime.Fork(binding, value.Effect.Intent, value.RuntimeRef)
	default:
		return nil, errors.New("runtime action is invalid")
	}
}

func commitEffect(op *experiment.Operation, value experiment.DecisionBoundEffect) error {
	existing, err := op.DecisionBoundEffect(value.Intent.ID)
	if err == nil {
		if reflect.DeepEqual(existing, value) {
			return nil
		}
		return errors.New("effect intent identity changed")
	}
	return op.CommitDecisionBoundEffect(value)
}

type stopRun struct {
	Action string                         `json:"action"`
	Effect experiment.DecisionBoundEffect `json:"effect"`
}

func (stopRun) toolName() string { return RunTool }
func (stopRun) runOperation()    {}
func (value stopRun) execute(binding Binding) (any, error) {
	if value.Effect.Intent.Kind != effect.Stop {
		return nil, errors.New("stop action requires stop effect")
	}
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	if err = commitEffect(op, value.Effect); err != nil {
		return nil, err
	}
	target, err := run.Open(binding.Root, binding.ExperimentID, value.Effect.Intent.RunID)
	if err != nil {
		return nil, err
	}
	return target.RequestStopEffect(value.Effect.Intent)
}

type pollRun struct {
	Action     string `json:"action"`
	RunID      string `json:"run_id"`
	RuntimeRef string `json:"runtime_ref"`
}

func (pollRun) toolName() string { return RunTool }
func (pollRun) runOperation()    {}
func (value pollRun) execute(binding Binding) (any, error) {
	if value.RunID == "" || value.RuntimeRef == "" || binding.Runtime == nil {
		return nil, errors.New("tool runtime profile is unavailable")
	}
	return binding.Runtime.Poll(binding, value.RunID, value.RuntimeRef)
}

type statusRun struct {
	Action string `json:"action"`
	RunID  string `json:"run_id"`
}

func (statusRun) toolName() string { return RunTool }
func (statusRun) runOperation()    {}
func (value statusRun) execute(binding Binding) (any, error) {
	if value.RunID == "" {
		return nil, errors.New("run id is required")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, value.RunID)
	if err != nil {
		return nil, err
	}
	return op.ProjectStatus(processidentity.SystemProber{})
}
