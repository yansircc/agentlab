package tool

import (
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
)

type bindRun struct {
	Action   string                             `json:"action"`
	Binding  experiment.DecisionBoundRunBinding `json:"binding"`
	Origin   experiment.RunOrigin               `json:"origin"`
	Prepared artifact.Ref                       `json:"prepared"`
}

func (bindRun) toolName() string { return ApplyTool }
func (bindRun) applyOperation()  {}

func (value bindRun) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	value.Binding.Decision = resolveDecisionEvidence(binding, value.Binding.Decision)
	return op.BindPreparedRunWithDecision(value.Binding, value.Origin, value.Prepared)
}

type recordFinding struct {
	Action string                          `json:"action"`
	Value  experiment.DecisionBoundFinding `json:"value"`
}

func (recordFinding) toolName() string { return ApplyTool }
func (recordFinding) applyOperation()  {}

func (value recordFinding) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	value.Value.Decision = resolveDecisionEvidence(binding, value.Value.Decision)
	if err := op.RecordFindingWithDecision(value.Value); err != nil {
		return nil, err
	}
	return op.Status()
}

type renderHandoff struct {
	Action     string                        `json:"action"`
	Decision   experiment.SupervisorDecision `json:"decision"`
	FindingIDs []string                      `json:"finding_ids"`
}

func (renderHandoff) toolName() string { return ApplyTool }
func (renderHandoff) applyOperation()  {}

func (value renderHandoff) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.RenderHandoffWithDecision(resolveDecisionEvidence(binding, value.Decision), value.FindingIDs)
}
