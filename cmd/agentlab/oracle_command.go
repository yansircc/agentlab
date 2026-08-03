package main

import (
	"context"
	"time"

	commandoracle "github.com/yansircc/agentlab/internal/oracle/command"
)

type commandOracleRequest struct {
	Command                  []string          `json:"command"`
	Directory                string            `json:"directory"`
	Timeout                  string            `json:"timeout"`
	MaxOutputBytes           int               `json:"max_output_bytes"`
	PublicEnvironment        map[string]string `json:"public_environment,omitempty"`
	SecretEnvironmentHandles map[string]string `json:"secret_environment_handles,omitempty"`
	SideEffects              []string          `json:"side_effects"`
}

func runCommandOracle(args []string) (any, error) {
	flags, err := parseOracleFlags("oracle command", args)
	if err != nil {
		return nil, err
	}
	var request commandOracleRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	timeout, err := time.ParseDuration(request.Timeout)
	if err != nil {
		return nil, err
	}
	return commandoracle.Execute(context.Background(), flags.store, commandoracle.Spec{
		Command: request.Command, Directory: request.Directory, Timeout: timeout,
		MaxOutputBytes: request.MaxOutputBytes, PublicEnvironment: request.PublicEnvironment,
		SecretEnvironmentHandles: request.SecretEnvironmentHandles, SideEffects: request.SideEffects,
	})
}
