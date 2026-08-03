package tool

import "github.com/yansircc/agentlab/internal/experiment"

type recordDiagnosis struct {
	Action string                            `json:"action"`
	Value  experiment.DecisionBoundDiagnosis `json:"value"`
}

func (recordDiagnosis) toolName() string { return ApplyTool }
func (recordDiagnosis) applyOperation()  {}

func (value recordDiagnosis) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	if err := op.RecordDiagnosisWithDecision(value.Value); err != nil {
		return nil, err
	}
	return op.Status()
}

type bindCandidate struct {
	Action string                            `json:"action"`
	Value  experiment.DecisionBoundCandidate `json:"value"`
}

func (bindCandidate) toolName() string { return ApplyTool }
func (bindCandidate) applyOperation()  {}

func (value bindCandidate) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	return op.BindCandidateWithDecision(value.Value)
}

type continueRun struct {
	Action string                           `json:"action"`
	Value  experiment.DecisionBoundContinue `json:"value"`
}

func (continueRun) toolName() string { return ApplyTool }
func (continueRun) applyOperation()  {}

func (value continueRun) execute(binding Binding) (any, error) {
	op, err := binding.experiment()
	if err != nil {
		return nil, err
	}
	if err := op.RecordContinueWithDecision(value.Value); err != nil {
		return nil, err
	}
	return op.Status()
}
