package tool

import (
	"errors"

	"github.com/yansircc/agentlab/internal/experiment"
)

type compareOperation interface {
	Operation
	compareOperation()
}

func decodeCompare(data []byte) (Operation, error) {
	action, err := decodeAction(data)
	if err != nil {
		return nil, err
	}
	switch action {
	case "record":
		var value recordComparison
		return decodeCompareValue(data, &value)
	case "gate_record":
		var value recordGate
		return decodeCompareValue(data, &value)
	case "show":
		var value showComparison
		return decodeCompareValue(data, &value)
	case "gate_show":
		var value showGate
		return decodeCompareValue(data, &value)
	default:
		return nil, errors.New("unknown compare action")
	}
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
