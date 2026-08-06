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
	commandPrefix []string
	commandPaths  map[string]string
	workspace     string
	runtimeRoot   string
}

func NewSandbox(spec SandboxSpec) (Sandbox, error) { return newSandbox(spec) }

func (s Sandbox) Wrap(command []string) ([]string, error) {
	if len(s.commandPrefix) == 0 || s.workspace == "" || s.runtimeRoot == "" || len(command) == 0 || command[0] == "" {
		return nil, errors.New("coder sandbox is invalid")
	}
	result := append([]string(nil), s.commandPrefix...)
	wrapped := append([]string(nil), command...)
	if s.commandPaths != nil {
		path := s.commandPaths[wrapped[0]]
		if path == "" {
			return nil, errors.New("coder command is outside sandbox authority")
		}
		wrapped[0] = path
	}
	return append(result, wrapped...), nil
}

func (s Sandbox) Workspace() string { return s.workspace }

func (s Sandbox) RuntimeRoot() string { return s.runtimeRoot }
