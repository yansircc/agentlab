package finding

import (
	"errors"
	"regexp"

	"github.com/yansircc/agentlab/internal/run"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type Finding struct {
	ID         string            `json:"id"`
	Class      string            `json:"class"`
	Severity   Severity          `json:"severity"`
	Symptom    string            `json:"symptom"`
	Impact     string            `json:"impact"`
	Evidence   []run.EvidenceRef `json:"evidence"`
	Confidence Confidence        `json:"confidence"`
	Falsifier  string            `json:"falsifier"`
}

func (f Finding) Validate() error {
	if !idPattern.MatchString(f.ID) || f.Class == "" || f.Symptom == "" || f.Impact == "" || f.Falsifier == "" || len(f.Evidence) == 0 || len(f.Evidence) > 100 {
		return errors.New("finding requires bounded evidence and complete symptom fields")
	}
	if len(f.Class) > 128 || len(f.Symptom) > 4096 || len(f.Impact) > 4096 || len(f.Falsifier) > 4096 {
		return errors.New("finding text exceeds handoff bounds")
	}
	if f.Severity != SeverityLow && f.Severity != SeverityMedium && f.Severity != SeverityHigh && f.Severity != SeverityCritical {
		return errors.New("finding severity is invalid")
	}
	if f.Confidence != ConfidenceLow && f.Confidence != ConfidenceMedium && f.Confidence != ConfidenceHigh {
		return errors.New("finding confidence is invalid")
	}
	seen := map[run.EvidenceRef]bool{}
	for _, ref := range f.Evidence {
		if ref.Sequence == 0 || ref.Item < 0 || seen[ref] {
			return errors.New("finding evidence reference is invalid or duplicated")
		}
		seen[ref] = true
	}
	return nil
}

type DispositionKind string

const (
	ConfirmedProductGap  DispositionKind = "confirmed_product_gap"
	WorkerError          DispositionKind = "worker_error"
	EnvironmentIssue     DispositionKind = "environment_issue"
	InsufficientEvidence DispositionKind = "insufficient_evidence"
	Duplicate            DispositionKind = "duplicate"
	ExperimentRequired   DispositionKind = "experiment_required"
	AcceptedRisk         DispositionKind = "accepted_risk"
)

type Disposition struct {
	FindingID string          `json:"finding_id"`
	Kind      DispositionKind `json:"kind"`
	Authority string          `json:"authority"`
	Reason    string          `json:"reason"`
}

func (d Disposition) Validate() error {
	if !idPattern.MatchString(d.FindingID) || d.Authority == "" || d.Reason == "" {
		return errors.New("disposition fields are required")
	}
	valid := d.Kind == ConfirmedProductGap || d.Kind == WorkerError || d.Kind == EnvironmentIssue || d.Kind == InsufficientEvidence || d.Kind == Duplicate || d.Kind == ExperimentRequired || d.Kind == AcceptedRisk
	if !valid || (d.Kind == AcceptedRisk && d.Authority != "human") {
		return errors.New("disposition kind or authority is invalid")
	}
	if len(d.Authority) > 128 || len(d.Reason) > 4096 {
		return errors.New("disposition text exceeds bounds")
	}
	return nil
}
