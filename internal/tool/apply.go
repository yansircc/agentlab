package tool

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/preparation"
)

type applyOperation interface {
	Operation
	applyOperation()
}

func decodeApply(data []byte) (Operation, error) {
	action, err := decodeAction(data)
	if err != nil {
		return nil, err
	}
	switch action {
	case "begin_preparation":
		var value beginPreparation
		return decodeApplyValue(data, &value)
	case "record_fact":
		var value recordFact
		return decodeApplyValue(data, &value)
	case "propose_preparation_decision":
		var value proposePreparationDecision
		return decodeApplyValue(data, &value)
	case "resolve_preparation_decision":
		var value resolvePreparationDecision
		return decodeApplyValue(data, &value)
	case "record_leakage_assay":
		var value recordLeakageAssay
		return decodeApplyValue(data, &value)
	case "challenge_basis", "seal_preparation", "begin_experiment":
		var value emptyApply
		return decodeApplyValue(data, &value)
	case "challenge":
		var value challengePreparation
		return decodeApplyValue(data, &value)
	case "bind_run":
		var value bindRun
		return decodeApplyValue(data, &value)
	case "record_finding":
		var value recordFinding
		return decodeApplyValue(data, &value)
	case "render_handoff":
		var value renderHandoff
		return decodeApplyValue(data, &value)
	case "record_diagnosis":
		var value recordDiagnosis
		return decodeApplyValue(data, &value)
	case "bind_candidate":
		var value bindCandidate
		return decodeApplyValue(data, &value)
	case "continue":
		var value continueRun
		return decodeApplyValue(data, &value)
	default:
		return nil, errors.New("unknown apply action")
	}
}

func decodeApplyValue(data []byte, value applyOperation) (Operation, error) {
	if err := strictDecode(data, value); err != nil {
		return nil, err
	}
	return value, nil
}

type emptyApply struct {
	Action string `json:"action"`
}

func (emptyApply) toolName() string { return ApplyTool }
func (emptyApply) applyOperation()  {}
func (value emptyApply) execute(binding Binding) (any, error) {
	switch value.Action {
	case "challenge_basis":
		op, err := binding.preparation()
		if err != nil {
			return nil, err
		}
		return op.ChallengeBasis()
	case "seal_preparation":
		op, err := binding.preparation()
		if err != nil {
			return nil, err
		}
		return op.Seal()
	case "begin_experiment":
		if binding.PreparationID == "" {
			return nil, errors.New("tool host has no preparation binding")
		}
		op, err := binding.experiment()
		if err != nil {
			return nil, err
		}
		return op.Begin(binding.PreparationID)
	default:
		return nil, errors.New("apply action is invalid")
	}
}

type beginPreparation struct {
	Action          string         `json:"action"`
	UserIntent      artifact.Ref   `json:"user_intent"`
	SourceSnapshot  artifact.Ref   `json:"source_snapshot"`
	PublicArtifacts []artifact.Ref `json:"public_artifacts,omitempty"`
}

func (beginPreparation) toolName() string { return ApplyTool }
func (beginPreparation) applyOperation()  {}
func (value beginPreparation) execute(binding Binding) (any, error) {
	op, err := binding.preparation()
	if err != nil {
		return nil, err
	}
	return op.BeginRefs(preparation.BeginRefsSpec{
		UserIntent: value.UserIntent, SourceSnapshot: value.SourceSnapshot,
		PublicArtifacts: value.PublicArtifacts, Authority: binding.authority(),
	})
}

type recordFact struct {
	Action string                     `json:"action"`
	Fact   preparation.RepositoryFact `json:"fact"`
}

func (recordFact) toolName() string { return ApplyTool }
func (recordFact) applyOperation()  {}
func (value recordFact) execute(binding Binding) (any, error) {
	op, err := binding.preparation()
	if err != nil {
		return nil, err
	}
	if err = op.RecordFact(value.Fact); err != nil {
		return nil, err
	}
	return op.Status()
}

type proposePreparationDecision struct {
	Action   string                   `json:"action"`
	Decision preparation.DecisionNode `json:"decision"`
}

func (proposePreparationDecision) toolName() string { return ApplyTool }
func (proposePreparationDecision) applyOperation()  {}
func (value proposePreparationDecision) execute(binding Binding) (any, error) {
	op, err := binding.preparation()
	if err != nil {
		return nil, err
	}
	if err = op.ProposeDecision(value.Decision); err != nil {
		return nil, err
	}
	return op.Status()
}

type resolvePreparationDecision struct {
	Action     string                 `json:"action"`
	Resolution preparation.Resolution `json:"resolution"`
}

func (resolvePreparationDecision) toolName() string { return ApplyTool }
func (resolvePreparationDecision) applyOperation()  {}
func (value resolvePreparationDecision) execute(binding Binding) (any, error) {
	op, err := binding.preparation()
	if err != nil {
		return nil, err
	}
	if err = op.ResolveDecision(value.Resolution); err != nil {
		return nil, err
	}
	return op.Status()
}
