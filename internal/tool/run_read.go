package tool

import (
	"errors"

	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
)

type pollRun struct {
	Action     string `json:"action"`
	RunID      string `json:"run_id"`
	RuntimeRef string `json:"runtime_ref"`
}

func (pollRun) toolName() string { return RunTool }
func (pollRun) runOperation()    {}
func (value pollRun) execute(binding Binding) (any, error) {
	if value.RunID == "" || value.RuntimeRef == "" || binding.Runtime == nil {
		return nil, errors.New("tool runtime profile is unavailable")
	}
	return binding.Runtime.Poll(binding, value.RunID, value.RuntimeRef)
}

type statusRun struct {
	Action string `json:"action"`
	RunID  string `json:"run_id"`
}

func (statusRun) toolName() string { return RunTool }
func (statusRun) runOperation()    {}
func (value statusRun) execute(binding Binding) (any, error) {
	if value.RunID == "" {
		return nil, errors.New("run id is required")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, value.RunID)
	if err != nil {
		return nil, err
	}
	if err := op.RequireManifest(); err != nil {
		return nil, err
	}
	return op.ProjectStatus(processidentity.SystemProber{})
}
