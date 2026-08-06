package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/coder"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

// PiLaunch is the Host-issued process authority shared by Pi roles.
type PiLaunch struct {
	NodePath                 string            `json:"node_path"`
	RuntimeRoot              string            `json:"runtime_root"`
	ReadOnlyRoots            []string          `json:"read_only_roots"`
	AllowedExecutables       []string          `json:"allowed_executables"`
	PublicEnvironment        map[string]string `json:"public_environment,omitempty"`
	SecretEnvironmentHandles map[string]string `json:"secret_environment_handles,omitempty"`
	AllowNetwork             bool              `json:"allow_network"`
}

func (value PiLaunch) Validate() error {
	if !filepath.IsAbs(value.NodePath) || !filepath.IsAbs(value.RuntimeRoot) {
		return errors.New("Pi launch is invalid")
	}
	for _, path := range append(append([]string{}, value.ReadOnlyRoots...), value.AllowedExecutables...) {
		if !filepath.IsAbs(path) {
			return errors.New("Pi launch path is invalid")
		}
	}
	reserved := map[string]bool{"HOME": true, "PATH": true, "TMPDIR": true, "PI_CODING_AGENT_DIR": true, "PI_CODING_AGENT_SESSION_DIR": true, "AGENTLAB_CONTEXT_FILTER_ONLY": true, "AGENTLAB_WORKER_FIXTURE": true, "AGENTLAB_WORKER_DEPLOYCTL": true}
	for key, value := range value.PublicEnvironment {
		if !environmentName(key) || reserved[key] || value == "" || len(value) > 65536 {
			return errors.New("Pi public environment is invalid")
		}
	}
	for key, handle := range value.SecretEnvironmentHandles {
		if !environmentName(key) || reserved[key] || !environmentName(handle) || len(handle) > 256 {
			return errors.New("Pi secret environment is invalid")
		}
		if _, exists := value.PublicEnvironment[key]; exists {
			return errors.New("Pi environment overlaps")
		}
	}
	return nil
}

func (value PiLaunch) clone() PiLaunch {
	result := value
	result.ReadOnlyRoots = append([]string(nil), value.ReadOnlyRoots...)
	result.AllowedExecutables = append([]string(nil), value.AllowedExecutables...)
	result.PublicEnvironment = cloneEnvironment(value.PublicEnvironment)
	result.SecretEnvironmentHandles = cloneEnvironment(value.SecretEnvironmentHandles)
	return result
}

func startPiCoder(binding Binding, operation *run.Operation, intent effect.Intent, profile PiRuntimeProfile, receipt run.CoderProfile) (any, error) {
	launch := *profile.CoderLaunch
	if err := preparePiRuntime(launch.RuntimeRoot, profile.SessionPath); err != nil {
		return nil, err
	}
	identity, err := canonicalPiIdentity(profile.Identity)
	if err != nil {
		return nil, err
	}
	sandbox, err := coder.NewSandbox(coder.SandboxSpec{
		Workspace: profile.CoderWorkspace, RuntimeRoot: launch.RuntimeRoot,
		ReadOnlyRoots: uniquePaths(append(launch.ReadOnlyRoots, identity.SDKRoot)),
		Executables:   uniquePaths(append([]string{launch.NodePath}, launch.AllowedExecutables...)), AllowNetwork: launch.AllowNetwork,
	})
	if err != nil {
		return nil, err
	}
	session, err := runtimeSession(sandbox.RuntimeRoot(), profile.SessionPath)
	if err != nil {
		return nil, err
	}
	workspace, err := coder.Open(binding.store(), receipt.CandidateWorkspace, receipt.SourceSnapshot, sandbox.Workspace())
	if err != nil {
		return nil, errors.New("coder workspace differs from Host capability")
	}
	handoff, err := binding.store().Read(receipt.Handoff)
	if err != nil || len(handoff) == 0 || len(handoff) > 32768 || !utf8.Valid(handoff) {
		return nil, errors.New("coder handoff is invalid")
	}
	environment, err := launch.environment(sandbox.RuntimeRoot())
	if err != nil {
		return nil, err
	}
	command, err := sandbox.Wrap(piCoderCommand(identity, launch.NodePath, session, sandbox.RuntimeRoot(), string(handoff)))
	if err != nil {
		return nil, err
	}
	return piadapter.BeginManagedEffect(operation, intent, session, profile.Policy, command, environment, sandbox.Workspace(), &receipt, func(code int) error {
		if _, err := piadapter.Poll(operation, session); err != nil {
			return fmt.Errorf("final Pi session drain: %w", err)
		}
		if code != 0 {
			return fmt.Errorf("coder process exited with code %d", code)
		}
		candidate, err := workspace.Seal(binding.store())
		if err != nil {
			return fmt.Errorf("seal coder workspace: %w", err)
		}
		if _, err := operation.RecordCoderCompletion(candidate); err != nil {
			return fmt.Errorf("record coder completion: %w", err)
		}
		return nil
	})
}

func preparePiRuntime(root, session string) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(session) || filepath.Base(session) == "." {
		return errors.New("coder runtime path is invalid")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return errors.New("coder runtime root is not empty")
	}
	return nil
}

func runtimeSession(root, session string) (string, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(session))
	if err != nil {
		return "", err
	}
	result := filepath.Join(parent, filepath.Base(session))
	if !inside(canonical, result) {
		return "", errors.New("coder session escapes runtime root")
	}
	return result, nil
}

func (value PiLaunch) environment(runtimeRoot string) ([]string, error) {
	values := cloneEnvironment(value.PublicEnvironment)
	values["HOME"], values["TMPDIR"], values["PATH"] = runtimeRoot, runtimeRoot, value.sandboxPath()
	values["PI_CODING_AGENT_DIR"], values["PI_CODING_AGENT_SESSION_DIR"] = runtimeRoot, runtimeRoot
	for key, handle := range value.SecretEnvironmentHandles {
		secret, exists := os.LookupEnv(handle)
		if !exists || secret == "" {
			return nil, errors.New("coder secret handle is unavailable")
		}
		values[key] = secret
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result, nil
}

// sandboxPath is Host-derived from the declared executable capability. It
// never inherits the Host PATH, so a Pi bash tool can discover only commands
// whose exact files the operating-system sandbox made reachable.
func (value PiLaunch) sandboxPath() string {
	directories := map[string]bool{"/bin": true}
	for _, executable := range append([]string{value.NodePath}, value.AllowedExecutables...) {
		if filepath.IsAbs(executable) {
			directories[filepath.Dir(executable)] = true
		}
	}
	result := make([]string, 0, len(directories))
	for directory := range directories {
		result = append(result, directory)
	}
	sort.Strings(result)
	return strings.Join(result, string(os.PathListSeparator))
}

// canonicalPiIdentity resolves the SDK root to its real path so the mount
// list and the Pi command agree inside the chroot. The Host may declare the
// SDK through a symlink; mounting the resolved directory while launching the
// unresolved path would make cli.js unreachable inside the sandbox.
func canonicalPiIdentity(identity piadapter.IdentityConfig) (piadapter.IdentityConfig, error) {
	resolved, err := filepath.EvalSymlinks(identity.SDKRoot)
	if err != nil || !filepath.IsAbs(resolved) {
		return identity, errors.New("Pi identity SDK root is invalid")
	}
	identity.SDKRoot = resolved
	return identity, nil
}

func piCoderCommand(identity piadapter.IdentityConfig, node, session, runtimeRoot, handoff string) []string {
	prompt := "You are the isolated Coder. Use only the bounded handoff below and repair the candidate through the public workspace.\n\n" + handoff
	return []string{node, filepath.Join(identity.SDKRoot, "dist", "cli.js"), "--session", session, "--session-dir", runtimeRoot, "--provider", identity.Provider, "--model", identity.Model, "--thinking", identity.ThinkingPolicy, "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--tools", "read,bash,edit,write,grep,find,ls", prompt}
}

func uniquePaths(values []string) []string {
	seen, result := map[string]bool{}, make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func cloneEnvironment(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func environmentName(value string) bool {
	if value == "" {
		return false
	}
	for index, item := range value {
		if !(item == '_' || item >= 'A' && item <= 'Z' || item >= 'a' && item <= 'z' || index > 0 && item >= '0' && item <= '9') {
			return false
		}
	}
	return true
}

func inside(root, value string) bool {
	relative, err := filepath.Rel(root, value)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func overlaps(left, right string) bool { return inside(left, right) || inside(right, left) }
