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
	HostRoot         string
	SkillRoot        string
	SDKRoot          string
	NodePath         string
	Provider         string
	Model            string
	ThinkingPolicy   string
	CompactionPolicy string
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
	canaryRef, err := bindLiveCanary(store, piadapter.LiveCanarySpec{NodePath: spec.NodePath, SDKRoot: spec.SDKRoot, ExtensionPath: extension, BinaryPath: binary, Identity: identity}, adapter, canary)
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
	profiles, err := runtimeProfiles(value, spec, identity, workspace, workspaceReceipt, capability)
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
	planPath := filepath.Join(spec.HostRoot, "pi-runtime-plan.json")
	if err := transaction.Replace(planPath, plan, 0o600); err != nil {
		return Preflight{}, err
	}
	supervisorPlan := tool.PiSupervisorPlan{
		Contract: tool.PiSupervisorPlanContract,
		Launch: tool.PiLaunch{
			NodePath: spec.NodePath, RuntimeRoot: filepath.Join(spec.HostRoot, "supervisor-runtime"),
			ReadOnlyRoots: []string{spec.SDKRoot, spec.SkillRoot}, AllowNetwork: true,
		},
		SessionPath: filepath.Join(spec.HostRoot, "supervisor-runtime", "session.jsonl"), SkillRoot: spec.SkillRoot, Identity: identityConfig,
		Binding: tool.PiSupervisorBinding{Root: value.EvaluatedRoot, PreparationID: value.PreparationID, ExperimentID: value.ExperimentID, RuntimePlanPath: planPath},
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

func runtimeProfiles(value Preflight, spec RuntimeSpec, identity piadapter.AdapterIdentity, workspace string, workspaceReceipt, capability artifact.Ref) ([]tool.PiRuntimeProfile, error) {
	if !filepath.IsAbs(spec.NodePath) || !workspaceReceipt.Valid() || !capability.Valid() {
		return nil, errors.New("deployctl runtime profile is invalid")
	}
	tools, err := executablePaths("go", "sh", "grep", "find", "ls")
	if err != nil {
		return nil, err
	}
	policy := run.StopPolicy{FirstEventTimeout: 2 * time.Second, SoftIdleTimeout: 2 * time.Minute, HardIdleTimeout: 5 * time.Minute, OwnsWorkerProcess: true}
	workerRuntime := filepath.Join(spec.HostRoot, "worker-runtime")
	coderRuntime := filepath.Join(spec.HostRoot, "coder-runtime")
	worker := tool.PiRuntimeProfile{
		Ref: "baseline-worker", ExperimentID: value.ExperimentID, RunID: value.BaselineRunID, Role: effect.WorkerStart, SessionPath: filepath.Join(workerRuntime, "session.jsonl"),
		Identity: piadapter.IdentityConfig{SDKRoot: spec.SDKRoot, ContextFilterPath: filepath.Join(spec.SkillRoot, "extension.ts"), AdapterDigest: identity.AdapterDigest, Provider: spec.Provider, Model: spec.Model, ThinkingPolicy: spec.ThinkingPolicy, CompactionPolicy: spec.CompactionPolicy},
		Policy:   policy, WorkerLaunch: &tool.PiWorkerLaunch{Launch: tool.PiLaunch{NodePath: spec.NodePath, RuntimeRoot: workerRuntime, ReadOnlyRoots: []string{spec.SDKRoot}, AllowNetwork: true}, FixtureRoot: value.Fixture.Root(), DeployctlExecutable: filepath.Join(value.EvaluatedRoot, "baseline-candidate", "bin", "deployctl"), CandidateExecutable: value.CandidateExecutable, WorkerInput: value.WorkerInput},
	}
	coder := tool.PiRuntimeProfile{
		Ref: "coder-repair", ExperimentID: value.ExperimentID, RunID: coderRunID, Role: effect.CoderStart, SessionPath: filepath.Join(coderRuntime, "session.jsonl"),
		Identity: worker.Identity, Policy: policy, CoderSourceSnapshot: value.SourceSnapshot, CoderWorkspaceReceipt: workspaceReceipt, CoderCapabilityProfile: capability, CoderWorkspace: workspace,
		CoderLaunch: &tool.PiLaunch{NodePath: spec.NodePath, RuntimeRoot: coderRuntime, ReadOnlyRoots: []string{spec.SDKRoot}, AllowedExecutables: tools, AllowNetwork: true},
	}
	if _, err := tool.NewPiRuntimeHost([]tool.PiRuntimeProfile{worker, coder}); err != nil {
		return nil, err
	}
	return []tool.PiRuntimeProfile{worker, coder}, nil
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

func executablePaths(names ...string) ([]string, error) {
	result := make([]string, 0, len(names))
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil || !filepath.IsAbs(path) {
			return nil, errors.New("required Coder executable is unavailable")
		}
		result = append(result, path)
	}
	return result, nil
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
