package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yansircc/agentlab/internal/run"
)

type PublicContractDetector struct {
	AllowedTools []string `json:"allowed_tools"`
}

func (detector PublicContractDetector) Detect(items []run.EvidenceItem) (*Finding, error) {
	if len(detector.AllowedTools) == 0 || len(detector.AllowedTools) > 100 || len(items) > 100 {
		return nil, errors.New("public-contract detector configuration or input is invalid")
	}
	allowed := map[string]bool{}
	for _, label := range detector.AllowedTools {
		if label == "" || allowed[label] {
			return nil, errors.New("allowed public tools are invalid or duplicated")
		}
		allowed[label] = true
	}
	refs := []run.EvidenceRef{}
	labels := []string{}
	for _, item := range items {
		if item.Kind == run.EvidenceToolCall && !allowed[item.Label] {
			refs = append(refs, item.Ref)
			labels = append(labels, item.Label)
		}
	}
	if len(refs) == 0 {
		return nil, nil
	}
	data, _ := json.Marshal(struct {
		Detector PublicContractDetector `json:"detector"`
		Evidence []run.EvidenceRef      `json:"evidence"`
	}{detector, refs})
	sum := sha256.Sum256(data)
	result := Finding{
		ID: "public-bypass-" + hex.EncodeToString(sum[:8]), Class: "public_contract_bypass", Severity: SeverityHigh,
		Symptom:  fmt.Sprintf("Worker invoked tools outside the declared public contract: %v", labels),
		Impact:   "the observed result depends on capabilities unavailable through the declared public interface",
		Evidence: refs, Confidence: ConfidenceHigh,
		Falsifier: "show that every cited tool label is included in the exact public contract bound to the run",
	}
	return &result, result.Validate()
}
