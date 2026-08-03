package tool

import "github.com/yansircc/agentlab/internal/preparation"

type recordLeakageAssay struct {
	Action string                   `json:"action"`
	Assay  preparation.LeakageAssay `json:"assay"`
}

func (recordLeakageAssay) toolName() string { return ApplyTool }
func (recordLeakageAssay) applyOperation()  {}

func (value recordLeakageAssay) execute(binding Binding) (any, error) {
	op, err := binding.preparation()
	if err != nil {
		return nil, err
	}
	if err := op.RecordLeakageAssay(value.Assay); err != nil {
		return nil, err
	}
	return op.Status()
}

type challengePreparation struct {
	Action    string                `json:"action"`
	Challenge preparation.Challenge `json:"challenge"`
}

func (challengePreparation) toolName() string { return ApplyTool }
func (challengePreparation) applyOperation()  {}

func (value challengePreparation) execute(binding Binding) (any, error) {
	op, err := binding.preparation()
	if err != nil {
		return nil, err
	}
	if err := op.Challenge(value.Challenge); err != nil {
		return nil, err
	}
	return op.Status()
}
