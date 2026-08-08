//go:build linux

package run

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/coder"
	"github.com/yansircc/agentlab/internal/effect"
)

// TestCoderSandboxRunRecordsTerminalFacts drives the real sandbox Coder pi
// through the managed run lifecycle and asserts the terminal facts are
// recorded, exactly the boundary the live trial never crossed.
func TestCoderSandboxRunRecordsTerminalFacts(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root for the Linux sandbox namespaces")
	}
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	entry, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}
	key := os.Getenv("XAI_API_KEY")
	if key == "" {
		t.Skip("XAI_API_KEY is not set")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".agentlab-coder-finish-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	sdkRoot := filepath.Dir(filepath.Dir(entry))
	skillRoot := filepath.Join(root, "skill")
	workspace, runtimeRoot := filepath.Join(root, "workspace"), filepath.Join(root, "runtime")
	for _, path := range []string{skillRoot, workspace, runtimeRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	repo := os.Getenv("AGENTLAB_REPO")
	if repo == "" {
		repo = "../.."
	}
	extensionData, err := os.ReadFile(filepath.Join(repo, "skills", "agentlab", "extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "extension.ts"), extensionData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(skillRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(skillRoot, "bin", "agentlab")
	build := exec.Command("go", "build", "-o", binary, "./cmd/agentlab")
	build.Dir = repo
	if err := build.Run(); err != nil {
		t.Skipf("bundled binary build is unavailable: %v", err)
	}
	tools := []string{node, shell}
	for _, name := range []string{"go", "grep", "find", "ls", "cat", "head", "tail", "sed", "awk", "mkdir", "cp", "mv", "rm", "wc", "diff", "touch", "chmod", "echo", "printf", "sort", "uniq", "cut", "tr", "basename", "dirname", "xargs"} {
		if path, err := exec.LookPath(name); err == nil {
			tools = append(tools, path)
		}
	}
	sandbox, err := coder.NewSandbox(coder.SandboxSpec{Workspace: workspace, RuntimeRoot: runtimeRoot, ReadOnlyRoots: []string{sdkRoot, skillRoot}, Executables: tools, AllowNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	op, err := Open(t.TempDir(), "repro-experiment", "coder-run")
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	bindTestManifest(t, op)
	t.Cleanup(func() { _ = os.RemoveAll(runtimeRoot) })
	payload, err := EncodeStartPayload(effect.WorkerStart, StartPayload{})
	if err != nil {
		t.Fatal(err)
	}
	startRef, err := op.artifacts.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(runtimeRoot, "session.jsonl")
	command := []string{node, entry, "--session", session, "--session-dir", runtimeRoot, "--provider", "xai", "--model", "grok-4.3", "--thinking", "high", "--no-extensions", "--extension", filepath.Join(skillRoot, "extension.ts"), "--no-builtin-tools", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--tools", "read,bash,edit,write,grep,find,ls", "--print", "Reply with exactly REPRO_OK and nothing else."}
	wrapped, err := sandbox.Wrap(command)
	if err != nil {
		t.Fatal(err)
	}
	policy := StopPolicy{FirstEventTimeout: 300 * time.Second, SoftIdleTimeout: 2 * time.Minute, HardIdleTimeout: 5 * time.Minute, OwnsWorkerProcess: true}
	finalized := make(chan int, 1)
	started, err := op.BeginManagedAttachedEffect(effect.Intent{ID: "coder-start", RunID: "coder-run", Kind: effect.WorkerStart, Payload: startRef}, ManagedAttachedSpec{
		Adapter: "pi-session-v3", Policy: policy, Capabilities: RequiredAdapterCapabilities(), Command: wrapped, Environment: []string{"HOME=" + runtimeRoot, "TMPDIR=" + runtimeRoot, "PATH=/usr/bin:/bin", "XAI_API_KEY=" + key, "PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CODING_AGENT_SESSION_DIR=" + runtimeRoot}, WorkingDirectory: workspace,
		Ready: func() (string, []byte, error) {
			deadline := time.Now().Add(300 * time.Second)
			for {
				if data, err := os.ReadFile(session); err == nil && len(data) > 0 {
					return readSessionID(data), nil, nil
				}
				if time.Now().After(deadline) {
					return "", nil, os.ErrDeadlineExceeded
				}
				time.Sleep(10 * time.Millisecond)
			}
		}, Finalize: func(code int) error {
			// The real Coder callback drains the session before finishing, so
			// reproduce that exactly.
			if _, pollErr := piadapter.Poll(op, session); pollErr != nil {
				finalized <- -2
				return pollErr
			}
			finalized <- code
			return nil
		},
	})
	if err != nil || started.Receipt.IntentID != "coder-start" {
		t.Fatalf("managed coder start = %#v, %v", started, err)
	}
	select {
	case code := <-finalized:
		t.Logf("coder exited with code %d", code)
	case <-time.After(7 * time.Minute):
		t.Fatal("coder finalizer did not run within 7 minutes")
	}
	time.Sleep(2 * time.Second)
	status, err := op.Status(nil)
	if err != nil {
		t.Fatal(err)
	}
	records, err := op.ledger.Replay()
	if err != nil {
		t.Fatal(err)
	}
	last := records[len(records)-1]
	t.Logf("final status health=%s last_event=%s", status.Health, last.Kind)
	if last.Kind != eventTerminalAccepted && last.Kind != eventTerminalRejected {
		t.Fatalf("coder run did not settle a terminal fact; last event = %s (health %s)", last.Kind, status.Health)
	}
}

func readSessionID(data []byte) string {
	for _, line := range bytes.Split(data, []byte("\n")) {
		var h struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(line, &h) != nil || h.ID == "" {
			continue
		}
		return h.ID
	}
	return ""
}
