package tool

import (
	"path/filepath"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

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
	value.Value.Decision = resolveDecisionEvidence(binding, value.Value.Decision)
	value.Value.Diagnosis = resolveSourceEvidence(binding, value.Value.Diagnosis)
	if err := op.RecordDiagnosisWithDecision(value.Value); err != nil {
		return nil, err
	}
	return op.Status()
}

// resolveSourceEvidence is Host resolution of the model's source citation: it
// binds each evidence path to the exact file artifact of the sealed snapshot,
// so the model needs only the path and line range, never an opaque file ref.
// The model sees the workspace under its absolute host path, while the sealed
// snapshot names files relative to the workspace root; both forms match, and
// the line range is clamped to the artifact's actual line count.
func resolveSourceEvidence(binding Binding, value diagnosis.Diagnosis) diagnosis.Diagnosis {
	snapshot, err := source.Load(binding.store(), value.SourceSnapshot)
	if err != nil {
		return value
	}
	for index := range value.SourceEvidence {
		evidence := &value.SourceEvidence[index]
		var target *source.File
		for candidate := range snapshot.Files {
			file := &snapshot.Files[candidate]
			if evidence.Path == file.Path || strings.HasSuffix(evidence.Path, "/"+file.Path) || filepath.Base(evidence.Path) == file.Path {
				target = file
				break
			}
		}
		if target == nil {
			continue
		}
		evidence.Path = target.Path
		evidence.Artifact = target.Artifact
		if data, err := binding.store().Read(target.Artifact); err == nil {
			if lines := strings.Count(string(data), "\n") + 1; lines > 0 {
				if evidence.EndLine > lines {
					evidence.EndLine = lines
				}
				if evidence.StartLine > lines {
					evidence.StartLine = lines
				}
			}
		}
	}
	return value
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
	value.Value.Decision = resolveDecisionEvidence(binding, value.Value.Decision)
	value.Value.CompletionRef = resolveCandidateCompletion(binding, value.Value.CoderRun, value.Value.CompletionRef)
	return op.BindCandidateWithDecision(value.Value)
}

// resolveCandidateCompletion is Host resolution of the model's Coder citation:
// the completion receipt is read from the named Coder run's ledger, so the
// model needs only the run id, never an opaque receipt ref.
func resolveCandidateCompletion(binding Binding, coderRun string, provided artifact.Ref) artifact.Ref {
	if coderRun == "" {
		return provided
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, coderRun)
	if err != nil {
		return provided
	}
	receipt, _, err := op.CoderCompletionReceipt()
	if err != nil {
		return provided
	}
	return receipt
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
