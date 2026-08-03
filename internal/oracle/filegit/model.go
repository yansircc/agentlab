package filegit

import (
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/oracle"
)

type Spec struct {
	Root          string   `json:"root"`
	Paths         []string `json:"paths"`
	MaxFileBytes  int64    `json:"max_file_bytes"`
	CaptureGit    bool     `json:"capture_git"`
	GitExecutable string   `json:"git_executable,omitempty"`
	MaxGitBytes   int      `json:"max_git_bytes,omitempty"`
	SideEffects   []string `json:"side_effects"`
}

type FileFact struct {
	Path       string        `json:"path"`
	Kind       string        `json:"kind"`
	Mode       uint32        `json:"mode,omitempty"`
	Size       int64         `json:"size,omitempty"`
	Content    *artifact.Ref `json:"content,omitempty"`
	LinkTarget string        `json:"link_target,omitempty"`
	Failure    string        `json:"failure,omitempty"`
}

type GitFact struct {
	Executable artifact.Ref `json:"executable"`
	Status     artifact.Ref `json:"status"`
	ExitCode   int          `json:"exit_code"`
	Failure    string       `json:"failure,omitempty"`
	Truncated  bool         `json:"truncated"`
}

type Output struct {
	Files []FileFact `json:"files"`
	Git   *GitFact   `json:"git,omitempty"`
}

type Result struct {
	Receipt oracle.Receipt `json:"receipt"`
	Output  Output         `json:"output"`
}
