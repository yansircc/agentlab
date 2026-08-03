package coder

import "errors"

// SandboxSpec is Host-private authority for one Coder process. It names every
// path the operating-system sandbox may make reachable.
type SandboxSpec struct {
	Workspace     string
	RuntimeRoot   string
	ReadOnlyRoots []string
	Executables   []string
	AllowNetwork  bool
}

type Sandbox struct {
	launcher    string
	profile     string
	workspace   string
	runtimeRoot string
}

func NewSandbox(spec SandboxSpec) (Sandbox, error) { return newSandbox(spec) }

func (s Sandbox) Wrap(command []string) ([]string, error) {
	if s.launcher == "" || s.profile == "" || s.workspace == "" || len(command) == 0 || command[0] == "" {
		return nil, errors.New("coder sandbox is invalid")
	}
	result := []string{s.launcher, "-f", s.profile}
	return append(result, command...), nil
}

func (s Sandbox) Workspace() string { return s.workspace }

func (s Sandbox) RuntimeRoot() string { return s.runtimeRoot }
