package main

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/diagnosis"
	"github.com/yansircc/agentlab/internal/experiment"
)

type diagnosisRequest struct {
	ExperimentID string              `json:"experiment_id"`
	Diagnosis    diagnosis.Diagnosis `json:"diagnosis"`
}

type candidateRequest struct {
	ExperimentID string       `json:"experiment_id"`
	CandidateID  string       `json:"candidate_id"`
	DiagnosisID  string       `json:"diagnosis_id"`
	Candidate    artifact.Ref `json:"candidate"`
}

func diagnoseRecord(args []string) (any, error) {
	flags, err := parsePrepareRequest("diagnose record", args)
	if err != nil {
		return nil, err
	}
	var request diagnosisRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	op, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	if err := op.RecordDiagnosis(request.Diagnosis); err != nil {
		return nil, err
	}
	return op.Status()
}

func diagnoseBindCandidate(args []string) (any, error) {
	flags, err := parsePrepareRequest("diagnose bind-candidate", args)
	if err != nil {
		return nil, err
	}
	var request candidateRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	if !request.Candidate.Valid() {
		return nil, errors.New("candidate source snapshot is required")
	}
	op, err := experiment.Open(flags.root, request.ExperimentID)
	if err != nil {
		return nil, err
	}
	return op.BindCandidate(request.CandidateID, request.DiagnosisID, request.Candidate)
}
