package tool

import (
	"errors"
	"reflect"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
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
	case "start":
		var value startRun
		return decodeRunValue(data, &value)
	case "stop":
		var value stopRun
		return decodeRunValue(data, &value)
	case "checkpoint":
		var value checkpointRun
		return decodeRunValue(data, &value)
	case "fork":
		var value forkRun
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

type startRun struct {
	Action     string                        `json:"action"`
	Decision   experiment.SupervisorDecision `json:"decision"`
	EffectID   string                        `json:"effect_id"`
	RunID      string                        `json:"run_id"`
	RuntimeRef string                        `json:"runtime_ref"`
	Handoff    *artifact.Ref                 `json:"handoff,omitempty"`
}

func (startRun) toolName() string { return RunTool }
func (startRun) runOperation()    {}
func (value startRun) execute(binding Binding) (any, error) {
	if binding.Runtime == nil || value.EffectID != value.Decision.ID {
		return nil, errors.New("start operation is invalid")
	}
	intent, err := binding.Runtime.StartIntent(binding, StartRequest{ID: value.EffectID, RunID: value.RunID, RuntimeRef: value.RuntimeRef, Handoff: value.Handoff})
	if err != nil {
		return nil, err
	}
	if err := commitEffectForDecision(binding, value.Decision, intent); err != nil {
		return nil, err
	}
	return binding.Runtime.Start(binding, intent, value.RuntimeRef)
}

type stopRun struct {
	Action   string                        `json:"action"`
	Decision experiment.SupervisorDecision `json:"decision"`
	EffectID string                        `json:"effect_id"`
	RunID    string                        `json:"run_id"`
	Reason   string                        `json:"reason"`
}

func (stopRun) toolName() string { return RunTool }
func (stopRun) runOperation()    {}
func (value stopRun) execute(binding Binding) (any, error) {
	if value.EffectID != value.Decision.ID || value.RunID == "" {
		return nil, errors.New("stop operation is invalid")
	}
	payload, err := run.EncodeStopPayload(run.StopPayload{Reason: value.Reason})
	if err != nil {
		return nil, err
	}
	ref, err := binding.store().Put(payload)
	if err != nil {
		return nil, err
	}
	intent := effect.Intent{ID: value.EffectID, RunID: value.RunID, Kind: effect.Stop, Payload: ref}
	if err := commitEffectForDecision(binding, value.Decision, intent); err != nil {
		return nil, err
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, value.RunID)
	if err != nil {
		return nil, err
	}
	return op.RequestStopEffect(intent)
}

type checkpointRun struct {
	Action       string                        `json:"action"`
	Decision     experiment.SupervisorDecision `json:"decision"`
	EffectID     string                        `json:"effect_id"`
	RunID        string                        `json:"run_id"`
	RuntimeRef   string                        `json:"runtime_ref"`
	EntryLocator string                        `json:"entry_locator"`
}

func (checkpointRun) toolName() string { return RunTool }
func (checkpointRun) runOperation()    {}
func (value checkpointRun) execute(binding Binding) (any, error) {
	if binding.Runtime == nil || value.EffectID != value.Decision.ID {
		return nil, errors.New("checkpoint operation is invalid")
	}
	intent, err := binding.Runtime.CheckpointIntent(binding, CheckpointRequest{ID: value.EffectID, RunID: value.RunID, RuntimeRef: value.RuntimeRef, EntryLocator: value.EntryLocator})
	if err != nil {
		return nil, err
	}
	if err := commitEffectForDecision(binding, value.Decision, intent); err != nil {
		return nil, err
	}
	return binding.Runtime.Checkpoint(binding, intent, value.RuntimeRef)
}

type forkRun struct {
	Action     string                        `json:"action"`
	Decision   experiment.SupervisorDecision `json:"decision"`
	EffectID   string                        `json:"effect_id"`
	RunID      string                        `json:"run_id"`
	RuntimeRef string                        `json:"runtime_ref"`
	Checkpoint artifact.Ref                  `json:"checkpoint"`
}

func (forkRun) toolName() string { return RunTool }
func (forkRun) runOperation()    {}
func (value forkRun) execute(binding Binding) (any, error) {
	if binding.Runtime == nil || value.EffectID != value.Decision.ID {
		return nil, errors.New("fork operation is invalid")
	}
	intent, err := binding.Runtime.ForkIntent(binding, ForkRequest{ID: value.EffectID, RunID: value.RunID, RuntimeRef: value.RuntimeRef, Checkpoint: value.Checkpoint})
	if err != nil {
		return nil, err
	}
	if err := commitEffectForDecision(binding, value.Decision, intent); err != nil {
		return nil, err
	}
	return binding.Runtime.Fork(binding, intent, value.RuntimeRef)
}

func commitEffectForDecision(binding Binding, decision experiment.SupervisorDecision, intent effect.Intent) error {
	op, err := binding.experiment()
	if err != nil {
		return err
	}
	value := experiment.DecisionBoundEffect{Decision: decision, Intent: intent}
	existing, err := op.DecisionBoundEffect(intent.ID)
	if err == nil {
		if reflect.DeepEqual(existing, value) {
			return nil
		}
		return errors.New("effect intent identity changed")
	}
	return op.CommitDecisionBoundEffect(value)
}
