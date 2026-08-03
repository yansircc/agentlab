package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yansircc/agentlab/internal/run"
)

type RepeatedLabelDetector struct {
	Label     string `json:"label"`
	Threshold int    `json:"threshold"`
}

func (d RepeatedLabelDetector) Detect(items []run.EvidenceItem) (*Finding, error) {
	if d.Label == "" || d.Threshold < 2 || d.Threshold > 100 || len(items) > 100 {
		return nil, errors.New("repeated-label detector configuration or input is invalid")
	}
	refs := make([]run.EvidenceRef, 0, len(items))
	for _, item := range items {
		if item.Label == d.Label {
			refs = append(refs, item.Ref)
		}
	}
	if len(refs) < d.Threshold {
		return nil, nil
	}
	id := deterministicID(d, refs)
	result := Finding{
		ID: "repeated-" + id[:16], Class: "repeated_observable_event", Severity: SeverityMedium,
		Symptom:  fmt.Sprintf("observable label %q occurred %d times", d.Label, len(refs)),
		Impact:   "the Worker spent repeated observable operations without establishing task success",
		Evidence: refs, Confidence: ConfidenceHigh,
		Falsifier: "show that the cited events are required distinct progress steps under the public task contract",
	}
	return &result, result.Validate()
}

func deterministicID(detector RepeatedLabelDetector, refs []run.EvidenceRef) string {
	data, _ := json.Marshal(struct {
		Detector RepeatedLabelDetector `json:"detector"`
		Evidence []run.EvidenceRef     `json:"evidence"`
	}{detector, refs})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
