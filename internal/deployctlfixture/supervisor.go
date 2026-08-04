package deployctlfixture

import (
	"errors"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/gate"
	"github.com/yansircc/agentlab/internal/metaaudit"
	"github.com/yansircc/agentlab/internal/tool"
)

// StartSupervisor is Host-only orchestration. Its fixed plan was issued at
// runtime preflight; no Supervisor-controlled operation or path reaches it.
func (value Preflight) StartSupervisor() (tool.PiSupervisorReceipt, error) {
	if err := value.verifyRuntime(); err != nil {
		return tool.PiSupervisorReceipt{}, errors.New("deployctl Supervisor preflight is unavailable")
	}
	return tool.StartPiSupervisor(value.supervisorPlanPath)
}

// SupervisorStatus reads a Host-private receipt without projecting it into
// evaluated evidence or the four provider tools.
func (value Preflight) SupervisorStatus() (tool.PiSupervisorStatus, error) {
	if err := value.verifyRuntime(); err != nil {
		return tool.PiSupervisorStatus{}, errors.New("deployctl Supervisor preflight is unavailable")
	}
	return tool.SupervisorStatus(value.supervisorPlanPath)
}

// EvaluateRecursiveGate adds the deployctl-specific mutation-B requirement to
// the generic recursive audit. The held-out artifact remains Host-owned; the
// Supervisor cannot provide it through the four-tool surface.
func (value Preflight) EvaluateRecursiveGate(candidateID, gateID string, heldout artifact.Ref) (metaaudit.RecursiveResult, error) {
	if err := value.verifyRuntime(); err != nil {
		return metaaudit.RecursiveResult{}, errors.New("deployctl recursive gate preflight is unavailable")
	}
	result, err := metaaudit.Evaluate(value.EvaluatedRoot, value.AuditRoot, metaaudit.RecursiveSpec{
		AuditID: value.AuditID, ExperimentID: value.ExperimentID, CandidateID: candidateID, GateID: gateID,
	})
	if err != nil || result.Verdict != gate.Pass {
		return result, err
	}
	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		return metaaudit.RecursiveResult{}, err
	}
	gateResult, err := op.Gate(gateID)
	if err != nil || gateResult.Receipt.CandidateID != candidateID || VerifyHeldoutArtifact(artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts")), heldout, gateResult.Receipt.Candidate) != nil {
		return metaaudit.RecursiveResult{Verdict: gate.Block}, nil
	}
	return result, nil
}
