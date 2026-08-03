package gate

import (
	"errors"
	"regexp"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/run"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

type ItemStatus string

const (
	Passed  ItemStatus = "passed"
	Blocked ItemStatus = "blocked"
)

type Item struct {
	ID         string             `json:"id"`
	Status     ItemStatus         `json:"status"`
	Statement  string             `json:"statement"`
	Impact     string             `json:"impact"`
	Evidence   []run.EvidenceRef  `json:"evidence"`
	Severity   finding.Severity   `json:"severity"`
	Confidence finding.Confidence `json:"confidence"`
	Falsifier  string             `json:"falsifier"`
}

type Spec struct {
	ID           string `json:"id"`
	CandidateID  string `json:"candidate_id"`
	ComparisonID string `json:"comparison_id,omitempty"`
	Items        []Item `json:"items"`
}

type Receipt struct {
	Spec
	Candidate artifact.Ref `json:"candidate"`
}

type Verdict string

const (
	Pass  Verdict = "pass"
	Block Verdict = "blocked"
)

type Result struct {
	Receipt Receipt `json:"receipt"`
	Verdict Verdict `json:"verdict"`
}

func (s Spec) Validate() error {
	if !idPattern.MatchString(s.ID) || s.CandidateID == "" || len(s.Items) == 0 || len(s.Items) > 100 {
		return errors.New("gate requires id, candidate, and bounded items")
	}
	seen := map[string]bool{}
	for _, item := range s.Items {
		if !idPattern.MatchString(item.ID) || seen[item.ID] || (item.Status != Passed && item.Status != Blocked) {
			return errors.New("gate item identity or status is invalid")
		}
		seen[item.ID] = true
		candidate := item.finding(s.ID)
		if err := candidate.Validate(); err != nil {
			return err
		}
	}
	if s.Verdict() == Pass && s.ComparisonID == "" {
		return errors.New("passing gate requires comparison")
	}
	return nil
}

func (s Spec) Verdict() Verdict {
	for _, item := range s.Items {
		if item.Status == Blocked {
			return Block
		}
	}
	return Pass
}

func (s Spec) BlockerFindings() []finding.Finding {
	result := []finding.Finding{}
	for _, item := range s.Items {
		if item.Status == Blocked {
			result = append(result, item.finding(s.ID))
		}
	}
	return result
}

func (item Item) finding(gateID string) finding.Finding {
	return finding.Finding{
		ID: gateID + "." + item.ID, Class: "gate_blocker", Severity: item.Severity,
		Symptom: item.Statement, Impact: item.Impact, Evidence: item.Evidence,
		Confidence: item.Confidence, Falsifier: item.Falsifier,
	}
}
