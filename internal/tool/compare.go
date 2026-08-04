package tool

import (
	"errors"

	"github.com/yansircc/agentlab/internal/experiment"
)

type compareOperation interface {
	Operation
	compareOperation()
}

var compareActionDecoder = map[string]func() compareOperation{
	"record":      func() compareOperation { return &recordComparison{} },
	"gate_record": func() compareOperation { return &recordGate{} },
	"show":        func() compareOperation { return &showComparison{} },
	"gate_show":   func() compareOperation { return &showGate{} },
}

func compareActionNames() []string { return actionNames(compareActionDecoder) }

func decodeCompare(data []byte) (Operation, error) {
	action, err := decodeAction(data)
	if err != nil {
		return nil, err
	}
	newValue, ok := compareActionDecoder[action]
	if !ok {
		return nil, errors.New("unknown compare action")
	}
	return decodeCompareValue(data, newValue())
}

func decodeCompareValue(data []byte, value compareOperation) (Operation, error) {
	if err := strictDecode(data, value); err != nil {
		return nil, err
	}
	return value, nil
}

type recordComparison struct {
	Action string                             `json:"action"`
	Value  experiment.DecisionBoundComparison `json:"value"`
}

func (recordComparison) toolName() string  { return CompareTool }
func (recordComparison) compareOperation() {}
func (value recordComparison) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.CompareWithDecision(value.Value)
}

type recordGate struct {
	Action string                       `json:"action"`
	Value  experiment.DecisionBoundGate `json:"value"`
}

func (recordGate) toolName() string  { return CompareTool }
func (recordGate) compareOperation() {}
func (value recordGate) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.RecordGateWithDecision(value.Value)
}

type showComparison struct {
	Action       string `json:"action"`
	ComparisonID string `json:"comparison_id"`
}

func (showComparison) toolName() string  { return CompareTool }
func (showComparison) compareOperation() {}
func (value showComparison) execute(binding Binding) (any, error) {
	if value.ComparisonID == "" {
		return nil, errors.New("comparison id is required")
	}
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.Comparison(value.ComparisonID)
}

type showGate struct {
	Action string `json:"action"`
	GateID string `json:"gate_id"`
}

func (showGate) toolName() string  { return CompareTool }
func (showGate) compareOperation() {}
func (value showGate) execute(binding Binding) (any, error) {
	if value.GateID == "" {
		return nil, errors.New("gate id is required")
	}
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.Gate(value.GateID)
}
