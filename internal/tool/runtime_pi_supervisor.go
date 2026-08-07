package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/strictjson"
	"github.com/yansircc/agentlab/internal/transaction"
)

const (
	PiSupervisorPlanContract    = "agentlab.pi-supervisor-plan.v1"
	piSupervisorReceiptContract = "agentlab.pi-supervisor-receipt.v1"
)

// PiSupervisorBinding is Host authority consumed only by the bundled
// extension. No provider request can select or replace one of these values.
type PiSupervisorBinding struct {
	Root            string `json:"root"`
	PreparationID   string `json:"preparation_id"`
	ExperimentID    string `json:"experiment_id"`
	RuntimePlanPath string `json:"runtime_plan_path"`
}

// PiSupervisorPlan is a Host-private launch plan, deliberately separate from
// PiRuntimeProfile: Supervisor creation is not a fifth provider effect.
type PiSupervisorPlan struct {
	Contract              string                   `json:"contract"`
	Launch                PiLaunch                 `json:"launch"`
	SessionPath           string                   `json:"session_path"`
	SkillRoot             string                   `json:"skill_root"`
	Identity              piadapter.IdentityConfig `json:"identity"`
	Binding               PiSupervisorBinding      `json:"binding"`
	RoleCredentialHandles []string                 `json:"role_credential_handles,omitempty"`
}

// PiSupervisorReceipt records only Host-private process identity and the
// derived four-tool set. It never becomes evaluated evidence or a tool result.
type PiSupervisorReceipt struct {
	Contract    string                   `json:"contract"`
	PlanDigest  string                   `json:"plan_digest"`
	Process     processidentity.Identity `json:"process"`
	ActiveTools []string                 `json:"active_tools"`
}

type PiSupervisorStatus struct {
	Receipt     PiSupervisorReceipt         `json:"receipt"`
	Observation processidentity.Observation `json:"observation"`
}

func (value PiSupervisorPlan) Validate() error {
	if value.Contract != PiSupervisorPlanContract || value.Launch.Validate() != nil || len(value.Launch.AllowedExecutables) != 0 || !filepath.IsAbs(value.SessionPath) || !filepath.IsAbs(value.SkillRoot) || !filepath.IsAbs(value.Identity.SDKRoot) || !filepath.IsAbs(value.Identity.ContextFilterPath) || !validSupervisorDigest(value.Identity.AdapterDigest) || !supervisorText(value.Identity.Provider) || !supervisorText(value.Identity.Model) || !supervisorText(value.Identity.ThinkingPolicy) || value.Identity.CompactionPolicy != "off" {
		return errors.New("Pi Supervisor plan is invalid")
	}
	if filepath.Clean(value.Identity.ContextFilterPath) != filepath.Join(filepath.Clean(value.SkillRoot), "extension.ts") || !inside(value.Launch.RuntimeRoot, value.SessionPath) || !samePaths(value.Launch.ReadOnlyRoots, []string{value.Identity.SDKRoot, value.SkillRoot}) {
		return errors.New("Pi Supervisor plan authority is invalid")
	}
	if !regularFile(filepath.Join(value.SkillRoot, "SKILL.md")) || !regularFile(value.Identity.ContextFilterPath) || !executableFile(filepath.Join(value.SkillRoot, "bin", "agentlab")) {
		return errors.New("Pi Supervisor bundle is invalid")
	}
	if !filepath.IsAbs(value.Binding.Root) || value.Binding.PreparationID == "" || value.Binding.ExperimentID == "" || !filepath.IsAbs(value.Binding.RuntimePlanPath) || !regularFile(value.Binding.RuntimePlanPath) || !inside(filepath.Dir(value.Binding.RuntimePlanPath), value.Launch.RuntimeRoot) || overlaps(value.Binding.Root, filepath.Dir(value.Binding.RuntimePlanPath)) {
		return errors.New("Pi Supervisor Host binding is invalid")
	}
	if err := value.validateRoleCredentialHandles(); err != nil {
		return err
	}
	return nil
}

// validateRoleCredentialHandles keeps the Worker and Coder credential handle
// names Host-declared, distinct, and disjoint from the Supervisor's own
// launch handle, so each bounded role resolves only its own credential.
func (value PiSupervisorPlan) validateRoleCredentialHandles() error {
	if len(value.RoleCredentialHandles) == 0 {
		return errors.New("Pi Supervisor role credential handles are invalid")
	}
	seen := map[string]bool{}
	for _, launchHandle := range value.Launch.SecretEnvironmentHandles {
		seen[launchHandle] = true
	}
	for _, handle := range value.RoleCredentialHandles {
		if !environmentName(handle) || seen[handle] {
			return errors.New("Pi Supervisor role credential handles are invalid")
		}
		seen[handle] = true
	}
	return nil
}

func EncodePiSupervisorPlan(value PiSupervisorPlan) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func LoadPiSupervisorPlan(path string) (PiSupervisorPlan, error) {
	if !filepath.IsAbs(path) {
		return PiSupervisorPlan{}, errors.New("Pi Supervisor plan path is invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PiSupervisorPlan{}, err
	}
	return decodePiSupervisorPlan(data)
}

// StartPiSupervisor starts the one Host-prepared Supervisor process. A stale
// or mismatched receipt is fail-closed: it never restarts a session under a
// reused identity.
func StartPiSupervisor(planPath string) (PiSupervisorReceipt, error) {
	if !filepath.IsAbs(planPath) {
		return PiSupervisorReceipt{}, errors.New("Pi Supervisor plan path is invalid")
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		return PiSupervisorReceipt{}, err
	}
	plan, err := decodePiSupervisorPlan(data)
	if err != nil {
		return PiSupervisorReceipt{}, err
	}
	return startPiSupervisor(planPath, plan, data, piadapter.VerifyRuntimeIdentity)
}

func SupervisorStatus(planPath string) (PiSupervisorStatus, error) {
	if !filepath.IsAbs(planPath) {
		return PiSupervisorStatus{}, errors.New("Pi Supervisor plan path is invalid")
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		return PiSupervisorStatus{}, err
	}
	plan, err := decodePiSupervisorPlan(data)
	if err != nil {
		return PiSupervisorStatus{}, err
	}
	receipt, err := loadPiSupervisorReceipt(receiptPath(planPath), planDigest(data), plan.Launch.NodePath)
	if err != nil {
		return PiSupervisorStatus{}, err
	}
	return PiSupervisorStatus{Receipt: receipt, Observation: (processidentity.SystemProber{}).Observe(receipt.Process)}, nil
}

func startPiSupervisor(planPath string, plan PiSupervisorPlan, planData []byte, verify func(piadapter.IdentityConfig) (piadapter.AdapterIdentity, error)) (PiSupervisorReceipt, error) {
	if !filepath.IsAbs(planPath) || plan.Validate() != nil || verify == nil {
		return PiSupervisorReceipt{}, errors.New("Pi Supervisor start is invalid")
	}
	identity, err := verify(plan.Identity)
	if err != nil || identity.AdapterDigest != plan.Identity.AdapterDigest || identity.Provider != plan.Identity.Provider || identity.Model != plan.Identity.Model || identity.ThinkingPolicy != plan.Identity.ThinkingPolicy || identity.CompactionPolicy != plan.Identity.CompactionPolicy {
		return PiSupervisorReceipt{}, errors.New("Pi Supervisor runtime identity differs from Host binding")
	}
	lease, err := transaction.Acquire(filepath.Join(filepath.Dir(planPath), "supervisor-launch.lock"))
	if err != nil {
		return PiSupervisorReceipt{}, err
	}
	defer lease.Release()

	digest := planDigest(planData)
	if receipt, err := loadPiSupervisorReceipt(receiptPath(planPath), digest, plan.Launch.NodePath); err == nil {
		if (processidentity.SystemProber{}).Observe(receipt.Process) == processidentity.Matches {
			return receipt, nil
		}
		return PiSupervisorReceipt{}, errors.New("Pi Supervisor has an unsettled prior receipt")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return PiSupervisorReceipt{}, err
	}
	if err := preparePiRuntime(plan.Launch.RuntimeRoot, plan.SessionPath); err != nil {
		return PiSupervisorReceipt{}, err
	}
	environment, err := plan.environment()
	if err != nil {
		return PiSupervisorReceipt{}, err
	}
	command := piSupervisorCommand(plan)
	process := exec.Command(command[0], command[1:]...)
	process.Dir, process.Env, process.Stdin, process.Stdout, process.Stderr = plan.Launch.RuntimeRoot, environment, nil, io.Discard, io.Discard
	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := process.Start(); err != nil {
		return PiSupervisorReceipt{}, err
	}
	pgid, err := syscall.Getpgid(process.Process.Pid)
	if err != nil {
		_ = process.Process.Kill()
		_ = process.Wait()
		return PiSupervisorReceipt{}, err
	}
	processIdentity, err := processidentity.Capture(process.Process.Pid, pgid, command[0])
	if err != nil {
		terminatePiSupervisor(process, pgid)
		return PiSupervisorReceipt{}, err
	}
	receipt := PiSupervisorReceipt{Contract: piSupervisorReceiptContract, PlanDigest: digest, Process: processIdentity, ActiveTools: ActiveToolNames()}
	receiptData, err := json.Marshal(receipt)
	if err != nil || receipt.Validate() != nil || transaction.WriteOnce(receiptPath(planPath), receiptData, 0o600) != nil {
		terminatePiSupervisor(process, pgid)
		return PiSupervisorReceipt{}, errors.New("Pi Supervisor receipt could not be persisted")
	}
	go func() { _ = process.Wait() }()
	return receipt, nil
}

func decodePiSupervisorPlan(data []byte) (PiSupervisorPlan, error) {
	var value PiSupervisorPlan
	if strictjson.Decode(data, &value) != nil || value.Validate() != nil {
		return PiSupervisorPlan{}, errors.New("Pi Supervisor plan is invalid")
	}
	return value, nil
}

func (value PiSupervisorReceipt) Validate() error {
	if value.Contract != piSupervisorReceiptContract || !validSupervisorDigest(value.PlanDigest) || !sameStrings(value.ActiveTools, ActiveToolNames()) || value.Process.PID <= 0 || value.Process.PGID <= 0 || value.Process.StartToken == "" || !validSupervisorDigest(value.Process.CommandHash) || !filepath.IsAbs(value.Process.Executable) {
		return errors.New("Pi Supervisor receipt is invalid")
	}
	return nil
}

func loadPiSupervisorReceipt(path, digest, executable string) (PiSupervisorReceipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PiSupervisorReceipt{}, err
	}
	var value PiSupervisorReceipt
	if strictjson.Decode(data, &value) != nil || value.Validate() != nil || value.PlanDigest != digest || value.Process.Executable != executable {
		return PiSupervisorReceipt{}, errors.New("Pi Supervisor receipt is invalid")
	}
	return value, nil
}

func (value PiSupervisorPlan) environment() ([]string, error) {
	result, err := value.Launch.environment(value.Launch.RuntimeRoot)
	if err != nil {
		return nil, err
	}
	// The Supervisor is the parent of every bounded role launch: the bundled
	// extension spawns the Host tool invocation that starts the Worker and
	// Coder, and those processes resolve their credential handles from the
	// inherited environment. Pass the role handle values through the
	// Supervisor process so each role authenticates with only its own
	// credential, never visible to the model or another role.
	for _, handle := range value.RoleCredentialHandles {
		secret, exists := os.LookupEnv(handle)
		if !exists || secret == "" {
			return nil, errors.New("Supervisor role credential handle is unavailable")
		}
		result = append(result, handle+"="+secret)
	}
	return append(result,
		"AGENTLAB_ROOT="+value.Binding.Root,
		"AGENTLAB_PREPARATION="+value.Binding.PreparationID,
		"AGENTLAB_EXPERIMENT="+value.Binding.ExperimentID,
		"AGENTLAB_PI_RUNTIME_PLAN="+value.Binding.RuntimePlanPath,
	), nil
}

func piSupervisorCommand(value PiSupervisorPlan) []string {
	// The skill is loaded by name for discovery, but pi's progressive
	// disclosure leaves only the name and description in context and expects
	// the agent to read the full file on demand. The Supervisor has no read
	// tool (exactly four AgentLab tools), so the full SKILL.md must be
	// appended into the system prompt directly.
	return []string{
		value.Launch.NodePath, filepath.Join(value.Identity.SDKRoot, "dist", "cli.js"),
		"--session", value.SessionPath, "--session-dir", value.Launch.RuntimeRoot,
		"--provider", value.Identity.Provider, "--model", value.Identity.Model, "--thinking", value.Identity.ThinkingPolicy,
		"--no-extensions", "--extension", value.Identity.ContextFilterPath,
		"--no-builtin-tools", "--no-skills", "--skill", filepath.Join(value.SkillRoot, "SKILL.md"),
		"--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve",
		"--append-system-prompt", filepath.Join(value.SkillRoot, "SKILL.md"),
		"--tools", strings.Join(ActiveToolNames(), ","), "--print",
		"You are the AgentLab Supervisor, a separate role from the Worker: you never deploy anything yourself; a bounded Worker agent runs deployctl and you supervise it. The Host has already sealed the preparation and begun the experiment; the baseline run \"baseline-worker\" (runtime profile ref \"baseline-worker\") is bound and unstarted. Do NOT call begin_preparation, seal_preparation, begin_experiment, record_fact, challenge, or any setup action. Inspect the experiment ledger, start the baseline worker with the bootstrap decision in the appended skill, poll until it exits, then complete the MANDATORY sequence: stop, record_finding, render_handoff, start coder-repair, poll the Coder to completion, record_diagnosis, bind_candidate, start fresh-worker, poll it, record the comparison, record the gate. The task is not complete until the gate is recorded; after any stop or start you MUST continue with the next step and never write a final answer. Runtime profile refs equal the run ids (baseline-worker, coder-repair, candidate-worker-fresh-worker). Run ids are short Host names, never digests.",
	}
}

func receiptPath(planPath string) string {
	return filepath.Join(filepath.Dir(planPath), "supervisor-receipt.json")
}

func planDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func terminatePiSupervisor(process *exec.Cmd, pgid int) {
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	_ = process.Wait()
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func executableFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

func samePaths(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]bool{}
	for _, value := range left {
		seen[filepath.Clean(value)] = true
	}
	if len(seen) != len(right) {
		return false
	}
	for _, value := range right {
		if !seen[filepath.Clean(value)] {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func supervisorText(value string) bool {
	return value != "" && len(value) <= 256 && !strings.HasPrefix(value, "-") && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func validSupervisorDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, item := range value {
		if !(item >= 'a' && item <= 'f' || item >= '0' && item <= '9') {
			return false
		}
	}
	return true
}
