package main

import (
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/finding"
)

type findingRequest struct {
	ExperimentID string          `json:"experiment_id"`
	Finding      finding.Finding `json:"finding"`
}

type dispositionRequest struct {
	ExperimentID string              `json:"experiment_id"`
	Disposition  finding.Disposition `json:"disposition"`
}

func reviewRecord(args []string) (any, error) {
	flags, err := parsePrepareRequest("review record", args)
	if err != nil {
		return nil, err
	}
	var request findingRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	if err := op.RecordFinding(request.Finding); err != nil {
		return nil, err
	}
	return op.Status()
}

func reviewDisposition(args []string) (any, error) {
	flags, err := parsePrepareRequest("review disposition", args)
	if err != nil {
		return nil, err
	}
	var request dispositionRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	if err := op.RecordDisposition(request.Disposition); err != nil {
		return nil, err
	}
	return op.Status()
}
