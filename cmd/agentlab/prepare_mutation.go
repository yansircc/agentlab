package main

import "github.com/yansircc/agentlab/internal/preparation"

type factRequest struct {
	PreparationID string                     `json:"preparation_id"`
	Fact          preparation.RepositoryFact `json:"fact"`
}

type decisionRequest struct {
	PreparationID string                   `json:"preparation_id"`
	Decision      preparation.DecisionNode `json:"decision"`
}

type resolutionRequest struct {
	PreparationID string                 `json:"preparation_id"`
	Resolution    preparation.Resolution `json:"resolution"`
}

type assayRequest struct {
	PreparationID string                   `json:"preparation_id"`
	Assay         preparation.LeakageAssay `json:"assay"`
}

func prepareRecordFact(args []string) (any, error) {
	flags, err := parsePrepareRequest("prepare record-fact", args)
	if err != nil {
		return nil, err
	}
	var request factRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := preparation.Open(flags.root, request.PreparationID)
	if err != nil {
		return nil, err
	}
	if err := op.RecordFact(request.Fact); err != nil {
		return nil, err
	}
	return op.Status()
}

func preparePropose(args []string) (any, error) {
	flags, err := parsePrepareRequest("prepare propose-decision", args)
	if err != nil {
		return nil, err
	}
	var request decisionRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := preparation.Open(flags.root, request.PreparationID)
	if err != nil {
		return nil, err
	}
	if err := op.ProposeDecision(request.Decision); err != nil {
		return nil, err
	}
	return op.Status()
}

func prepareResolve(args []string) (any, error) {
	flags, err := parsePrepareRequest("prepare resolve", args)
	if err != nil {
		return nil, err
	}
	var request resolutionRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := preparation.Open(flags.root, request.PreparationID)
	if err != nil {
		return nil, err
	}
	if err := op.ResolveDecision(request.Resolution); err != nil {
		return nil, err
	}
	return op.Status()
}

func prepareAssay(args []string) (any, error) {
	flags, err := parsePrepareRequest("prepare assay", args)
	if err != nil {
		return nil, err
	}
	var request assayRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := preparation.Open(flags.root, request.PreparationID)
	if err != nil {
		return nil, err
	}
	if err := op.RecordLeakageAssay(request.Assay); err != nil {
		return nil, err
	}
	return op.Status()
}
