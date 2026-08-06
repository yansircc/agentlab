//go:build darwin

package coder

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const sandboxExecutable = "/usr/bin/sandbox-exec"

func newSandbox(spec SandboxSpec) (Sandbox, error) {
	workspace, err := sandboxDirectory(spec.Workspace)
	if err != nil {
		return Sandbox{}, err
	}
	runtimeRoot, err := sandboxDirectory(spec.RuntimeRoot)
	if err != nil || overlaps(workspace, runtimeRoot) {
		return Sandbox{}, errors.New("coder sandbox directories are invalid")
	}
	readOnly, err := sandboxDirectories(spec.ReadOnlyRoots)
	if err != nil || len(readOnly) == 0 {
		return Sandbox{}, errors.New("coder sandbox read roots are invalid")
	}
	for _, root := range readOnly {
		if overlaps(root, workspace) || overlaps(root, runtimeRoot) {
			return Sandbox{}, errors.New("coder sandbox roots overlap")
		}
	}
	executables, err := sandboxExecutables(spec.Executables)
	if err != nil || len(executables) == 0 {
		return Sandbox{}, errors.New("coder sandbox executables are invalid")
	}
	if _, err := os.Stat(sandboxExecutable); err != nil {
		return Sandbox{}, err
	}
	profile := filepath.Join(runtimeRoot, "agentlab-coder.sb")
	if err := os.WriteFile(profile, []byte(sandboxProfile(workspace, runtimeRoot, readOnly, executables, spec.AllowNetwork)), 0o600); err != nil {
		return Sandbox{}, err
	}
	return Sandbox{commandPrefix: []string{sandboxExecutable, "-f", profile}, workspace: workspace, runtimeRoot: runtimeRoot}, nil
}

func sandboxProfile(workspace, runtimeRoot string, readOnly, executables []string, allowNetwork bool) string {
	paths := append(append([]string{}, readOnly...), workspace, runtimeRoot)
	metadata := parentPaths(paths)
	var lines []string
	lines = append(lines, "(version 1)", "(deny default)", "(import \"system.sb\")", "(allow process-fork)")
	if allowNetwork {
		lines = append(lines, "(allow network-outbound)")
	}
	for _, path := range executables {
		lines = append(lines, allowLiteral("process-exec", path))
	}
	for _, path := range metadata {
		lines = append(lines, allowLiteral("file-read-metadata", path))
	}
	for _, path := range readOnly {
		lines = append(lines, allowSubpath("file-read*", path))
	}
	for _, path := range []string{workspace, runtimeRoot} {
		lines = append(lines, allowSubpath("file-read*", path), allowSubpath("file-write*", path))
	}
	return strings.Join(lines, "\n") + "\n"
}

func parentPaths(paths []string) []string {
	seen := map[string]bool{}
	for _, path := range paths {
		for current := filepath.Clean(path); current != "/"; current = filepath.Dir(current) {
			seen[current] = true
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func allowSubpath(operation, path string) string {
	return "(allow " + operation + " (subpath \"" + path + "\"))"
}

func allowLiteral(operation, path string) string {
	return "(allow " + operation + " (literal \"" + path + "\"))"
}
