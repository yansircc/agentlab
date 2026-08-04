package tool

import (
	"errors"
	"sort"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/preparation"
)

var applyActionDecoder = map[string]func() applyOperation{
	"begin_preparation":            func() applyOperation { return &beginPreparation{} },
	"record_fact":                  func() applyOperation { return &recordFact{} },
	"propose_preparation_decision": func() applyOperation { return &proposePreparationDecision{} },
	"resolve_preparation_decision": func() applyOperation { return &resolvePreparationDecision{} },
	"record_leakage_assay":         func() applyOperation { return &recordLeakageAssay{} },
	"challenge_basis":              func() applyOperation { return &emptyApply{} },
	"seal_preparation":             func() applyOperation { return &emptyApply{} },
	"begin_experiment":             func() applyOperation { return &emptyApply{} },
	"challenge":                    func() applyOperation { return &challengePreparation{} },
	"bind_run":                     func() applyOperation { return &bindRun{} },
	"record_finding":               func() applyOperation { return &recordFinding{} },
	"render_handoff":               func() applyOperation { return &renderHandoff{} },
	"record_diagnosis":             func() applyOperation { return &recordDiagnosis{} },
	"bind_candidate":               func() applyOperation { return &bindCandidate{} },
	"record_intervention":          func() applyOperation { return &recordIntervention{} },
	"continue":                     func() applyOperation { return &continueRun{} },
}

func applyActionNames() []string { return actionNames(applyActionDecoder) }

type applyOperation interface {
	Operation
	applyOperation()
}

func decodeApply(data []byte) (Operation, error) {
	action, err := decodeAction(data)
	if err != nil {
		return nil, err
	}
	newValue, ok := applyActionDecoder[action]
	if !ok {
		return nil, errors.New("unknown apply action")
	}
	return decodeApplyValue(data, newValue())
}

func decodeApplyValue(data []byte, value applyOperation) (Operation, error) {
	if err := strictDecode(data, value); err != nil {
		return nil, err
	}
	return value, nil
}

func actionNames[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for action := range values {
		result = append(result, action)
	}
	sort.Strings(result)
	return result
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
