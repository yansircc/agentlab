package deployctlfixture

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/coder"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/tool"
	"github.com/yansircc/agentlab/internal/transaction"
)

// RuntimeSpec is supplied by the Host after it has built dist/skill. It is
// never decoded from a Supervisor tool request or persisted in an evaluated
// artifact, because it contains process and filesystem authority.
type RuntimeSpec struct {
	HostRoot                   string
	SkillRoot                  string
	SDKRoot                    string
	NodePath                   string
	Provider                   string
	Model                      string
	ThinkingPolicy             string
	CompactionPolicy           string
	ProviderCredentialEnv      string
	WorkerCredentialHandle     string
	CoderCredentialHandle      string
	SupervisorCredentialHandle string
}

type runtimeCredentials struct {
	environment string
	worker      string
	coder       string
	supervisor  string
}

func (value RuntimeSpec) credentials() (runtimeCredentials, error) {
	result := runtimeCredentials{environment: value.ProviderCredentialEnv, worker: value.WorkerCredentialHandle, coder: value.CoderCredentialHandle, supervisor: value.SupervisorCredentialHandle}
	if !runtimeEnvironmentName(result.environment) || !runtimeEnvironmentName(result.worker) || !runtimeEnvironmentName(result.coder) || !runtimeEnvironmentName(result.supervisor) || result.worker == result.coder || result.worker == result.supervisor || result.coder == result.supervisor {
		return runtimeCredentials{}, errors.New("deployctl provider credential binding is invalid")
	}
	return result, nil
}

func (value runtimeCredentials) workerEnvironment() map[string]string {
	return map[string]string{value.environment: value.worker}
}

func (value runtimeCredentials) coderEnvironment() map[string]string {
	return map[string]string{value.environment: value.coder}
}

func (value runtimeCredentials) supervisorEnvironment() map[string]string {
	return map[string]string{value.environment: value.supervisor}
}

func verifyRuntimeCredentialIsolation(worker, coder, supervisor tool.PiLaunch) bool {
	values := []map[string]string{worker.SecretEnvironmentHandles, coder.SecretEnvironmentHandles, supervisor.SecretEnvironmentHandles}
	var environment string
	handles := map[string]bool{}
	for _, value := range values {
		if len(value) != 1 {
			return false
		}
		for key, handle := range value {
			if !runtimeEnvironmentName(key) || !runtimeEnvironmentName(handle) || (environment != "" && environment != key) || handles[handle] {
				return false
			}
			environment, handles[handle] = key, true
		}
	}
	return environment != "" && len(handles) == len(values)
}

// BindRuntime completes Stage 0's deterministic Host assembly. It binds the
// exact bundled binary and Pi identity into the baseline manifest and writes
// the private runtime plan used later by the bundled extension.
func (value Preflight) BindRuntime(spec RuntimeSpec) (Preflight, error) {
	return value.bindRuntime(spec, piadapter.RunLiveCanary)
}

func (value Preflight) bindRuntime(spec RuntimeSpec, canary liveCanaryRunner) (Preflight, error) {
	if err := value.verifyProvision(); err != nil || !newHostRoot(spec.HostRoot, value.EvaluatedRoot, value.AuditRoot, spec.SkillRoot, spec.SDKRoot) {
		return Preflight{}, errors.New("deployctl runtime preflight is invalid")
	}
	credentials, err := spec.credentials()
	if err != nil {
		return Preflight{}, err
	}
	binary, extension, err := bundledPaths(spec.SkillRoot)
	if err != nil {
		return Preflight{}, err
	}
	digest, err := fileDigest(binary)
	if err != nil {
		return Preflight{}, err
	}
	identityConfig := piadapter.IdentityConfig{
		SDKRoot: spec.SDKRoot, ContextFilterPath: extension, AdapterDigest: digest,
		Provider: spec.Provider, Model: spec.Model, ThinkingPolicy: spec.ThinkingPolicy, CompactionPolicy: spec.CompactionPolicy,
	}
	identity, err := piadapter.DiscoverIdentity(identityConfig)
	if err != nil {
		return Preflight{}, err
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	adapter, err := putCanonical(store, identity)
	if err != nil {
		return Preflight{}, err
	}
	canaryRef, err := bindLiveCanary(store, piadapter.LiveCanarySpec{NodePath: spec.NodePath, SDKRoot: spec.SDKRoot, ExtensionPath: extension, BinaryPath: binary, ProviderCredentialEnv: credentials.environment, CredentialHandle: credentials.worker, Identity: identity}, adapter, canary)
	if err != nil {
		return Preflight{}, err
	}
	if err := os.Mkdir(spec.HostRoot, 0o700); err != nil {
		return Preflight{}, err
	}
	workspace := filepath.Join(spec.HostRoot, "coder-workspace")
	workspaceReceipt, err := coder.Prepare(store, value.SourceSnapshot, workspace)
	if err != nil {
		return Preflight{}, err
	}
	capability, err := putCanonical(store, map[string]any{"contract": "agentlab.deployctl-coder-capability.v1", "source_snapshot": value.SourceSnapshot, "workspace": "host-bound"})
	if err != nil {
		return Preflight{}, err
	}
	profiles, err := runtimeProfiles(value, spec, credentials, identity, workspace, workspaceReceipt, capability)
	if err != nil {
		return Preflight{}, err
	}
	plan, err := tool.EncodePiRuntimePlan(profiles)
	if err != nil {
		return Preflight{}, err
	}
	runtime, err := recordRuntimeBinding(store, runtimeBinding{Contract: runtimeBindingContract, Adapter: canaryRef, CandidateExecutable: value.CandidateExecutable, WorkerProfile: "baseline-worker", CoderProfile: "coder-repair"})
	if err != nil {
		return Preflight{}, err
	}
	inputs, reset, err := preflightInputs(store, baselineRunID, value.reset, value.Candidate, canaryRef, runtime)
	if err != nil {
		return Preflight{}, err
	}
	coderInputs, _, err := preflightInputs(store, coderRunID, value.reset, value.Candidate, canaryRef, runtime)
	if err != nil {
		return Preflight{}, err
	}
	coderPrepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: coderRunID, Inputs: coderInputs})
	if err != nil {
		return Preflight{}, err
	}
	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		return Preflight{}, err
	}
	if _, err := op.Begin(value.PreparationID); err != nil {
		return Preflight{}, err
	}
	prepared, err := experiment.RecordPreparedRun(store, experiment.PreparedRun{Contract: experiment.PreparedRunContract, RunID: value.BaselineRunID, Inputs: inputs})
	if err != nil {
		return Preflight{}, err
	}
	if _, err := op.BindPreparedRun(value.BaselineRunID, experiment.NewFreshOrigin(), prepared); err != nil {
		return Preflight{}, err
	}
	// The Coder run is bound here too: its prepared-run ref is an opaque
	// Host artifact that the Supervisor tools never project, so the Coder
	// must be startable directly with just its handoff decision.
	if _, err := op.BindPreparedRun(coderRunID, experiment.NewFreshOrigin(), coderPrepared); err != nil {
		return Preflight{}, err
	}
	planPath := filepath.Join(spec.HostRoot, "pi-runtime-plan.json")
	if err := transaction.Replace(planPath, plan, 0o600); err != nil {
		return Preflight{}, err
	}
	supervisorPlan := tool.PiSupervisorPlan{
		Contract: tool.PiSupervisorPlanContract,
		Launch: tool.PiLaunch{
			NodePath: spec.NodePath, RuntimeRoot: filepath.Join(spec.HostRoot, "supervisor-runtime"),
			ReadOnlyRoots: []string{spec.SDKRoot, spec.SkillRoot}, SecretEnvironmentHandles: credentials.supervisorEnvironment(), AllowNetwork: true,
		},
		SessionPath: filepath.Join(spec.HostRoot, "supervisor-runtime", "session.jsonl"), SkillRoot: spec.SkillRoot, Identity: identityConfig,
		Binding: tool.PiSupervisorBinding{Root: value.EvaluatedRoot, PreparationID: value.PreparationID, ExperimentID: value.ExperimentID, RuntimePlanPath: planPath},
		// The Supervisor spawns the bounded Worker and Coder launches; their
		// credential handle values pass through its process environment only.
		RoleCredentialHandles: []string{credentials.worker, credentials.coder},
	}
	supervisorData, err := tool.EncodePiSupervisorPlan(supervisorPlan)
	if err != nil {
		return Preflight{}, err
	}
	supervisorPath := filepath.Join(spec.HostRoot, "supervisor-plan.json")
	if err := transaction.WriteOnce(supervisorPath, supervisorData, 0o600); err != nil {
		return Preflight{}, err
	}
	if err := writeRuntimePreflightLocator(spec.HostRoot, value.EvaluatedRoot, value.AuditRoot, coderPrepared); err != nil {
		return Preflight{}, err
	}
	value.FixtureReset, value.LiveCanary, value.CoderPrepared, value.Inputs, value.hostRoot, value.runtimePlanPath, value.supervisorPlanPath = reset, canaryRef, coderPrepared, inputs, spec.HostRoot, planPath, supervisorPath
	return value, value.Verify()
}

func runtimeProfiles(value Preflight, spec RuntimeSpec, credentials runtimeCredentials, identity piadapter.AdapterIdentity, workspace string, workspaceReceipt, capability artifact.Ref) ([]tool.PiRuntimeProfile, error) {
	if !filepath.IsAbs(spec.NodePath) || !workspaceReceipt.Valid() || !capability.Valid() {
		return nil, errors.New("deployctl runtime profile is invalid")
	}
	tools, err := executablePaths(spec.HostRoot, "go", "sh", "grep", "find", "ls", "cat", "head", "tail", "sed", "awk", "mkdir", "cp", "mv", "rm", "wc", "diff", "touch", "chmod", "echo", "printf", "sort", "uniq", "cut", "tr", "basename", "dirname", "xargs")
	if err != nil {
		return nil, err
	}
	// The sandboxed Pi role must boot node, load the pinned CLI and both
	// extensions, and write its session before the first event; a two-second
	// budget is too tight on a loaded runner, so the first event window is
	// generous while idle and hard limits stay bounded.
	policy := run.StopPolicy{FirstEventTimeout: 300 * time.Second, SoftIdleTimeout: 2 * time.Minute, HardIdleTimeout: 5 * time.Minute, OwnsWorkerProcess: true}
	workerRuntime := filepath.Join(spec.HostRoot, "worker-runtime")
	coderRuntime := filepath.Join(spec.HostRoot, "coder-runtime")
	worker := tool.PiRuntimeProfile{
		Ref: "baseline-worker", ExperimentID: value.ExperimentID, RunID: value.BaselineRunID, Role: effect.WorkerStart, SessionPath: filepath.Join(workerRuntime, "session.jsonl"),
		Identity: piadapter.IdentityConfig{SDKRoot: spec.SDKRoot, ContextFilterPath: filepath.Join(spec.SkillRoot, "extension.ts"), AdapterDigest: identity.AdapterDigest, Provider: spec.Provider, Model: spec.Model, ThinkingPolicy: spec.ThinkingPolicy, CompactionPolicy: spec.CompactionPolicy},
		Policy:   policy, WorkerLaunch: &tool.PiWorkerLaunch{Launch: tool.PiLaunch{NodePath: spec.NodePath, RuntimeRoot: workerRuntime, ReadOnlyRoots: []string{spec.SDKRoot}, SecretEnvironmentHandles: credentials.workerEnvironment(), AllowNetwork: true}, FixtureRoot: value.Fixture.Root(), DeployctlExecutable: filepath.Join(value.EvaluatedRoot, "baseline-candidate", "bin", "deployctl"), CandidateExecutable: value.CandidateExecutable, WorkerInput: value.WorkerInput, HostOracle: tool.HostOracleDeployctl},
	}
	coder := tool.PiRuntimeProfile{
		Ref: "coder-repair", ExperimentID: value.ExperimentID, RunID: coderRunID, Role: effect.CoderStart, SessionPath: filepath.Join(coderRuntime, "session.jsonl"),
		Identity: worker.Identity, Policy: policy, CoderSourceSnapshot: value.SourceSnapshot, CoderWorkspaceReceipt: workspaceReceipt, CoderCapabilityProfile: capability, CoderWorkspace: workspace,
		CoderLaunch: &tool.PiLaunch{NodePath: spec.NodePath, RuntimeRoot: coderRuntime, ReadOnlyRoots: []string{spec.SDKRoot}, AllowedExecutables: tools, SecretEnvironmentHandles: credentials.coderEnvironment(), AllowNetwork: true},
	}
	if _, err := tool.NewPiRuntimeHost([]tool.PiRuntimeProfile{worker, coder}); err != nil {
		return nil, err
	}
	return []tool.PiRuntimeProfile{worker, coder}, nil
}

func runtimeEnvironmentName(value string) bool {
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

func bundledPaths(root string) (string, string, error) {
	if !filepath.IsAbs(root) {
		return "", "", errors.New("bundled skill root is invalid")
	}
	binary, extension := filepath.Join(root, "bin", "agentlab"), filepath.Join(root, "extension.ts")
	for _, path := range []string{filepath.Join(root, "SKILL.md"), extension, binary} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", "", errors.New("bundled skill artifact is incomplete")
		}
	}
	return binary, extension, nil
}

// executablePaths resolves the Coder tools, re-homing the Go toolchain under
// the Host root: hosted-runner tool caches reject sandbox bind mounts, so the
// Go binary and its GOROOT are copied once to a bindable location at preflight.
func executablePaths(hostRoot string, names ...string) ([]string, error) {
	result := make([]string, 0, len(names))
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil || !filepath.IsAbs(path) {
			return nil, errors.New("required Coder executable is unavailable")
		}
		if name != "go" {
			result = append(result, path)
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		goBin := filepath.Join(hostRoot, "toolchain", "bin", "go")
		if err := os.MkdirAll(filepath.Dir(goBin), 0o700); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(goBin, data, 0o755); err != nil {
			return nil, err
		}
		goroot := filepath.Dir(filepath.Dir(resolved))
		target := filepath.Join(hostRoot, "toolchain")
		if err := copyTree(goroot, target); err != nil {
			return nil, err
		}
		result = append(result, goBin)
	}
	return result, nil
}

// copyTree copies a directory tree without ownership preservation; the Host
// copies are namespace-root owned, which the sandbox binds read-only.
func copyTree(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	for _, entry := range entries {
		from, to := filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := copyTree(from, to); err != nil {
				return err
			}
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(from)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, to); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func newHostRoot(root string, disjoint ...string) bool {
	if !filepath.IsAbs(root) {
		return false
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	for _, path := range disjoint {
		if !disjointRoots(root, path) {
			return false
		}
	}
	return true
}

func fileDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 64<<20 {
		return "", errors.New("bundled binary is invalid")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
