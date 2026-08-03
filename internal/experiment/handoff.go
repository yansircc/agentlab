package experiment

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
)

type HandoffResult struct {
	Artifact      artifact.Ref `json:"artifact"`
	FindingCount  int          `json:"finding_count"`
	EvidenceCount int          `json:"evidence_count"`
}

type HandoffRecord struct {
	Artifact   artifact.Ref `json:"artifact"`
	FindingIDs []string     `json:"finding_ids"`
}

func (value HandoffRecord) Validate() error {
	if !validRef(value.Artifact) || len(value.FindingIDs) == 0 || len(value.FindingIDs) > 50 {
		return errors.New("handoff record is invalid")
	}
	seen := map[string]bool{}
	for _, id := range value.FindingIDs {
		if !idPattern.MatchString(id) || seen[id] {
			return errors.New("handoff finding ids are invalid")
		}
		seen[id] = true
	}
	return nil
}

func (o *Operation) RenderHandoff(findingIDs []string) (HandoffResult, error) {
	current, err := o.current()
	if err != nil {
		return HandoffResult{}, err
	}
	return o.renderHandoff(current, findingIDs)
}

func (o *Operation) renderHandoff(current state, findingIDs []string) (HandoffResult, error) {
	if len(findingIDs) == 0 || len(findingIDs) > 50 {
		return HandoffResult{}, errors.New("handoff requires 1..50 findings")
	}
	seen := map[string]bool{}
	var document strings.Builder
	document.WriteString("# AgentLab Coder Handoff\n\n")
	document.WriteString("Evidence-only findings. Source owner, root cause, and repair boundary are intentionally absent.\n")
	evidenceCount := 0
	for _, id := range findingIDs {
		value := current.findings[id]
		if value.ID == "" || seen[id] {
			return HandoffResult{}, errors.New("handoff finding is absent or duplicated")
		}
		seen[id] = true
		fmt.Fprintf(&document, "\n## %s\n\nClass: %q\n\nSeverity: %q\n\nSymptom: %q\n\nImpact: %q\n\nConfidence: %q\n\nFalsifier: %q\n\nEvidence:\n", value.ID, value.Class, value.Severity, value.Symptom, value.Impact, value.Confidence, value.Falsifier)
		for _, ref := range value.Evidence {
			runOperation, err := run.Open(o.root, o.id, ref.RunID)
			if err != nil {
				return HandoffResult{}, err
			}
			item, err := runOperation.EvidenceAt(ref)
			if err != nil {
				return HandoffResult{}, err
			}
			fmt.Fprintf(&document, "\n- `%s/%s#%d.%d` kind=%q label=%q", ref.ExperimentID, ref.RunID, ref.Sequence, ref.Item, item.Kind, item.Label)
			if item.Raw.Digest != "" {
				fmt.Fprintf(&document, " raw=`sha256:%s`", item.Raw.Digest)
			}
			evidenceCount++
		}
		document.WriteString("\n")
	}
	ref, err := o.artifacts.Put([]byte(document.String()))
	if err != nil {
		return HandoffResult{}, err
	}
	return HandoffResult{Artifact: ref, FindingCount: len(findingIDs), EvidenceCount: evidenceCount}, nil
}
