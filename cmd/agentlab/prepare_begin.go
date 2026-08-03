package main

import (
	"errors"

	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/source"
)

type sourceFileRequest struct {
	Path        string `json:"path"`
	ContentPath string `json:"content_path"`
}

type beginPreparationRequest struct {
	PreparationID       string              `json:"preparation_id"`
	UserIntentPath      string              `json:"user_intent_path"`
	SourceFiles         []sourceFileRequest `json:"source_files"`
	PublicArtifactPaths []string            `json:"public_artifact_paths,omitempty"`
	Authority           string              `json:"authority"`
}

func prepareBegin(args []string) (any, error) {
	flags, err := parsePrepareRequest("prepare begin", args)
	if err != nil {
		return nil, err
	}
	var request beginPreparationRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	if request.PreparationID == "" || request.Authority == "" {
		return nil, errors.New("preparation id and authority are required")
	}
	intent, err := readRequiredFile(request.UserIntentPath, "user intent")
	if err != nil {
		return nil, err
	}
	sourceFiles := make([]source.InputFile, 0, len(request.SourceFiles))
	for _, file := range request.SourceFiles {
		content, err := readRequiredFile(file.ContentPath, "source file")
		if err != nil {
			return nil, err
		}
		sourceFiles = append(sourceFiles, source.InputFile{Path: file.Path, Content: content})
	}
	public := make([][]byte, 0, len(request.PublicArtifactPaths))
	for _, path := range request.PublicArtifactPaths {
		data, err := readRequiredFile(path, "public artifact")
		if err != nil {
			return nil, err
		}
		public = append(public, data)
	}
	op, err := preparation.Open(flags.root, request.PreparationID)
	if err != nil {
		return nil, err
	}
	return op.Begin(preparation.BeginSpec{UserIntent: intent, SourceFiles: sourceFiles, PublicArtifacts: public, Authority: request.Authority})
}
