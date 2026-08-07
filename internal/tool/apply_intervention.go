package tool

import "github.com/yansircc/agentlab/internal/experiment"

type recordIntervention struct {
	Action string                               `json:"action"`
	Value  experiment.DecisionBoundIntervention `json:"value"`
}

func (recordIntervention) toolName() string { return ApplyTool }
func (recordIntervention) applyOperation()  {}

func (value recordIntervention) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	value.Value.Decision = resolveDecisionEvidence(binding, value.Value.Decision)
	return op.RecordInterventionWithDecision(value.Value)
}
