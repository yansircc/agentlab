package metaaudit

import (
	"errors"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/gate"
)

type RecursiveSpec struct {
	AuditID      string `json:"audit_id"`
	ExperimentID string `json:"experiment_id"`
	CandidateID  string `json:"candidate_id"`
	GateID       string `json:"gate_id"`
}

type RecursiveResult struct {
	Verdict gate.Verdict `json:"verdict"`
}

func Evaluate(evaluatedRoot, auditRoot string, spec RecursiveSpec) (RecursiveResult, error) {
	if !filepath.IsAbs(evaluatedRoot) || !filepath.IsAbs(auditRoot) || !idPattern.MatchString(spec.AuditID) || !idPattern.MatchString(spec.ExperimentID) || !idPattern.MatchString(spec.CandidateID) || !idPattern.MatchString(spec.GateID) {
		return RecursiveResult{}, errors.New("recursive gate specification is invalid")
	}
	audit, err := Open(auditRoot, spec.AuditID)
	if err != nil {
		return RecursiveResult{}, err
	}
	state, err := audit.current()
	if err != nil || state.trial == nil || !state.sealed || state.intervened || state.trial.ExperimentID != spec.ExperimentID || state.trial.EvaluatedScope != artifact.NewStore(filepath.Join(evaluatedRoot, "artifacts")).Scope() {
		return RecursiveResult{Verdict: gate.Block}, nil
	}
	for _, finding := range state.findings {
		if err := verifyFinding(evaluatedRoot, *state.trial, finding); err != nil {
			return RecursiveResult{Verdict: gate.Block}, nil
		}
	}
	experimentOp, err := experiment.Open(evaluatedRoot, spec.ExperimentID)
	if err != nil {
		return RecursiveResult{}, err
	}
	settlement, err := experimentOp.EffectSettlement()
	if err != nil || len(settlement.Pending) != 0 || len(settlement.Orphan) != 0 || len(settlement.Mismatched) != 0 {
		return RecursiveResult{Verdict: gate.Block}, nil
	}
	result, err := experimentOp.Gate(spec.GateID)
	if err != nil || result.Verdict != gate.Pass || result.Receipt.CandidateID != spec.CandidateID {
		return RecursiveResult{Verdict: gate.Block}, nil
	}
	if _, err := experimentOp.GateDecision(spec.GateID); err != nil {
		return RecursiveResult{Verdict: gate.Block}, nil
	}
	return RecursiveResult{Verdict: gate.Pass}, nil
}
