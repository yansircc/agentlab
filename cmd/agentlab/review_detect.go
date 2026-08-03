package main

import (
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/finding"
	"github.com/yansircc/agentlab/internal/run"
)

type detectRepeatedRequest struct {
	ExperimentID string            `json:"experiment_id"`
	Label        string            `json:"label"`
	Threshold    int               `json:"threshold"`
	Evidence     []run.EvidenceRef `json:"evidence"`
}

type detectBypassRequest struct {
	ExperimentID string            `json:"experiment_id"`
	AllowedTools []string          `json:"allowed_tools"`
	Evidence     []run.EvidenceRef `json:"evidence"`
}

func reviewDetectRepeated(args []string) (any, error) {
	flags, err := parsePrepareRequest("review detect-repeated", args)
	if err != nil {
		return nil, err
	}
	var request detectRepeatedRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	items, err := loadEvidence(flags.root, request.ExperimentID, request.Evidence)
	if err != nil {
		return nil, err
	}
	value, err := (finding.RepeatedLabelDetector{Label: request.Label, Threshold: request.Threshold}).Detect(items)
	if err != nil || value == nil {
		return value, err
	}
	experimentOperation, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	if err := experimentOperation.RecordFinding(*value); err != nil {
		return nil, err
	}
	return value, nil
}

func reviewDetectBypass(args []string) (any, error) {
	flags, err := parsePrepareRequest("review detect-bypass", args)
	if err != nil {
		return nil, err
	}
	var request detectBypassRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	items, err := loadEvidence(flags.root, request.ExperimentID, request.Evidence)
	if err != nil {
		return nil, err
	}
	value, err := (finding.PublicContractDetector{AllowedTools: request.AllowedTools}).Detect(items)
	if err != nil || value == nil {
		return value, err
	}
	operation, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	if err := operation.RecordFinding(*value); err != nil {
		return nil, err
	}
	return value, nil
}

func loadEvidence(root, experimentID string, refs []run.EvidenceRef) ([]run.EvidenceItem, error) {
	items := make([]run.EvidenceItem, 0, len(refs))
	for _, ref := range refs {
		op, err := run.Open(root, experimentID, ref.RunID)
		if err != nil {
			return nil, err
		}
		item, err := op.EvidenceAt(ref)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
