package command

import (
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/oracle"
)

type Spec struct {
	Command                  []string          `json:"command"`
	Directory                string            `json:"directory"`
	Timeout                  time.Duration     `json:"timeout"`
	MaxOutputBytes           int               `json:"max_output_bytes"`
	PublicEnvironment        map[string]string `json:"public_environment,omitempty"`
	SecretEnvironmentHandles map[string]string `json:"secret_environment_handles,omitempty"`
	SideEffects              []string          `json:"side_effects"`
}

type Output struct {
	Executable artifact.Ref `json:"executable"`
	Stdout     artifact.Ref `json:"stdout"`
	Stderr     artifact.Ref `json:"stderr"`
	ExitCode   int          `json:"exit_code"`
	Failure    string       `json:"failure,omitempty"`
	Truncated  bool         `json:"truncated"`
}

type Result struct {
	Receipt oracle.Receipt `json:"receipt"`
	Output  Output         `json:"output"`
}
