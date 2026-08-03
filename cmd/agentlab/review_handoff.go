package main

import "github.com/yansircc/agentlab/internal/experiment"

type handoffRequest struct {
	ExperimentID string   `json:"experiment_id"`
	FindingIDs   []string `json:"finding_ids"`
}

func reviewHandoff(args []string) (any, error) {
	flags, err := parsePrepareRequest("review handoff", args)
	if err != nil {
		return nil, err
	}
	var request handoffRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	return op.RenderHandoff(request.FindingIDs)
}
