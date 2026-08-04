package main

import (
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
)

type bindRunRequest struct {
	ExperimentID string               `json:"experiment_id"`
	RunID        string               `json:"run_id"`
	Origin       experiment.RunOrigin `json:"origin"`
	Prepared     artifact.Ref         `json:"prepared"`
}

func experimentBindRun(args []string) (any, error) {
	flags, err := parsePrepareRequest("experiment bind-run", args)
	if err != nil {
		return nil, err
	}
	var request bindRunRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	return op.BindPreparedRun(request.RunID, request.Origin, request.Prepared)
}
