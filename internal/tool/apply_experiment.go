package tool

import "github.com/yansircc/agentlab/internal/experiment"

type bindRun struct {
	Action  string                             `json:"action"`
	Binding experiment.DecisionBoundRunBinding `json:"binding"`
	Origin  experiment.RunOrigin               `json:"origin"`
	Inputs  experiment.RunInputs               `json:"inputs"`
}

func (bindRun) toolName() string { return ApplyTool }
func (bindRun) applyOperation()  {}

func (value bindRun) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.BindRunWithDecision(value.Binding, value.Origin, value.Inputs)
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
	return op.RenderHandoffWithDecision(value.Decision, value.FindingIDs)
}
