package main

import "github.com/yansircc/agentlab/internal/preparation"

type challengeRequest struct {
	PreparationID string                `json:"preparation_id"`
	Challenge     preparation.Challenge `json:"challenge"`
}

func prepareChallenge(args []string) (any, error) {
	flags, err := parsePrepareRequest("prepare challenge", args)
	if err != nil {
		return nil, err
	}
	var request challengeRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := preparation.Open(flags.root, request.PreparationID)
	if err != nil {
		return nil, err
	}
	if err := op.Challenge(request.Challenge); err != nil {
		return nil, err
	}
	return op.Status()
}
