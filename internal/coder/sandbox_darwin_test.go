//go:build darwin

package coder

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxRequiresSeparateNonTemporaryCapabilities(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	runtime := filepath.Join(root, "runtime")
	packageRoot := filepath.Join(root, "sdk")
	node := filepath.Join(root, "node")
	for _, path := range []string{workspace, runtime, packageRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(node, []byte("node"), 0o700); err != nil {
		t.Fatal(err)
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtime, ReadOnlyRoots: []string{packageRoot}, Executables: []string{node}})
	if err != nil {
		t.Fatal(err)
	}
	command, err := sandbox.Wrap([]string{node, "cli.js"})
	if err != nil || command[0] != sandboxExecutable || sandbox.Workspace() != resolvedWorkspace {
		t.Fatalf("sandbox command = %#v, %v", command, err)
	}
	profile, err := os.ReadFile(filepath.Join(runtime, "agentlab-coder.sb"))
	if err != nil || !strings.Contains(string(profile), "(deny default)") || !strings.Contains(string(profile), packageRoot) || strings.Contains(string(profile), "/tmp") {
		t.Fatalf("sandbox profile = %q, %v", profile, err)
	}
	if _, err := NewSandbox(SandboxSpec{Workspace: "/tmp/workspace", RuntimeRoot: runtime, ReadOnlyRoots: []string{packageRoot}, Executables: []string{node}}); err == nil {
		t.Fatal("temporary workspace was accepted")
	}
	if _, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: workspace, ReadOnlyRoots: []string{packageRoot}, Executables: []string{node}}); err == nil {
		t.Fatal("shared runtime root was accepted")
	}
}

func TestSandboxRunsPinnedPiAndDeniesSiblingRoot(t *testing.T) {
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Dir(filepath.Dir(entry))
	node := filepath.Clean(filepath.Join(packageRoot, "..", "..", "..", "..", "bin", "node"))
	installation := filepath.Dir(filepath.Dir(node))
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-sandbox-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	workspace, runtimeRoot, audit := filepath.Join(root, "workspace"), filepath.Join(root, "runtime"), filepath.Join(root, "audit")
	for _, path := range []string{workspace, runtimeRoot, audit} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "input.txt"), []byte("workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audit, "private.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{installation}, Executables: []string{node}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := sandbox.Wrap([]string{node, entry, "--version"})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(version[0], version[1:]...)
	command.Dir = sandbox.Workspace()
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "0.83.0" {
		t.Fatalf("sandboxed Pi = %q, %v", output, err)
	}
	probe := `const fs=require("fs");const w=process.argv[1],a=process.argv[2];fs.writeFileSync(w+"/out.txt",fs.readFileSync(w+"/input.txt"));try{fs.readFileSync(a);process.exit(2)}catch(e){if(e.code!=="EPERM"&&e.code!=="EACCES")process.exit(3)}`
	wrapped, err := sandbox.Wrap([]string{node, "-e", probe, workspace, filepath.Join(audit, "private.txt")})
	if err != nil {
		t.Fatal(err)
	}
	command = exec.Command(wrapped[0], wrapped[1:]...)
	command.Dir = sandbox.Workspace()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sandboxed capability probe = %q, %v", output, err)
	}
}
