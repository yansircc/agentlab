package tool

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/transaction"
)

func TestPiSupervisorPlanFixesBundledAuthority(t *testing.T) {
	plan, _ := testPiSupervisorPlan(t)
	command := strings.Join(piSupervisorCommand(plan), "\x00")
	for _, required := range []string{"--no-extensions", plan.Identity.ContextFilterPath, "--no-builtin-tools", "--no-skills", filepath.Join(plan.SkillRoot, "SKILL.md"), "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--print", strings.Join(ActiveToolNames(), ",")} {
		if !strings.Contains(command, required) {
			t.Fatalf("Supervisor command omitted %q: %q", required, command)
		}
	}
	if strings.Count(command, "--extension") != 1 || strings.Count(command, "--skill") != 1 || strings.Contains(command, "bash,read,edit,write") {
		t.Fatalf("Supervisor command widened authority: %q", command)
	}
	environment, err := plan.environment()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(environment, "\x00")
	for _, required := range []string{"AGENTLAB_ROOT=" + plan.Binding.Root, "AGENTLAB_PREPARATION=" + plan.Binding.PreparationID, "AGENTLAB_EXPERIMENT=" + plan.Binding.ExperimentID, "AGENTLAB_PI_RUNTIME_PLAN=" + plan.Binding.RuntimePlanPath} {
		if !strings.Contains(joined, required) {
			t.Fatalf("Supervisor environment omitted Host binding %q", required)
		}
	}
	plan.Launch.AllowedExecutables = []string{"/bin/sh"}
	if plan.Validate() == nil {
		t.Fatal("Supervisor plan accepted a generic executable capability")
	}
}

func TestPiSupervisorStartWritesOneReceiptAndRefusesReuse(t *testing.T) {
	plan, planPath := testPiSupervisorPlan(t)
	data, err := EncodePiSupervisorPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.WriteOnce(planPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	verify := func(value piadapter.IdentityConfig) (piadapter.AdapterIdentity, error) {
		if value != plan.Identity {
			return piadapter.AdapterIdentity{}, errors.New("unexpected identity")
		}
		return piadapter.AdapterIdentity{AdapterDigest: value.AdapterDigest, Provider: value.Provider, Model: value.Model, ThinkingPolicy: value.ThinkingPolicy, CompactionPolicy: value.CompactionPolicy}, nil
	}
	receipt, err := startPiSupervisor(planPath, plan, data, verify)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = syscall.Kill(-receipt.Process.PGID, syscall.SIGKILL)
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) && (processidentity.SystemProber{}).Observe(receipt.Process) == processidentity.Matches {
			time.Sleep(10 * time.Millisecond)
		}
	}()
	if receipt.Validate() != nil || !sameStrings(receipt.ActiveTools, ActiveToolNames()) {
		t.Fatalf("Supervisor receipt = %#v", receipt)
	}
	status, err := SupervisorStatus(planPath)
	if err != nil || status.Observation != processidentity.Matches || !reflect.DeepEqual(status.Receipt, receipt) {
		t.Fatalf("Supervisor status = %#v, %v", status, err)
	}
	replayed, err := startPiSupervisor(planPath, plan, data, verify)
	if err != nil || !reflect.DeepEqual(replayed, receipt) {
		t.Fatalf("Supervisor launch replay = %#v, %v", replayed, err)
	}
	changed := append([]byte(nil), data...)
	changed = append(changed, '\n')
	if _, err := startPiSupervisor(planPath, plan, changed, verify); err == nil {
		t.Fatal("Supervisor start reused a receipt for a different plan identity")
	}
}

func testPiSupervisorPlan(t *testing.T) (PiSupervisorPlan, string) {
	t.Helper()
	parent := t.TempDir()
	host, evaluated := filepath.Join(parent, "host"), filepath.Join(parent, "evaluated")
	sdk, skill := filepath.Join(parent, "sdk"), filepath.Join(parent, "skill")
	for _, path := range []string{host, evaluated, filepath.Join(sdk, "dist"), filepath.Join(skill, "bin")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sdk, "dist", "cli.js"), []byte("#!/bin/sh\nsleep 20\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]struct {
		data []byte
		mode os.FileMode
	}{
		filepath.Join(skill, "SKILL.md"):            {[]byte("---\nname: agentlab\n---\n"), 0o600},
		filepath.Join(skill, "extension.ts"):        {[]byte("export default async () => {};\n"), 0o600},
		filepath.Join(skill, "bin", "agentlab"):     {[]byte("binary"), 0o700},
		filepath.Join(host, "pi-runtime-plan.json"): {[]byte("runtime plan"), 0o600},
	} {
		if err := os.WriteFile(path, data.data, data.mode); err != nil {
			t.Fatal(err)
		}
	}
	planPath := filepath.Join(host, "supervisor-plan.json")
	identity := piadapter.IdentityConfig{SDKRoot: sdk, ContextFilterPath: filepath.Join(skill, "extension.ts"), AdapterDigest: strings.Repeat("a", 64), Provider: "provider", Model: "model", ThinkingPolicy: "off", CompactionPolicy: "off"}
	plan := PiSupervisorPlan{
		Contract:    PiSupervisorPlanContract,
		Launch:      PiLaunch{NodePath: "/bin/sh", RuntimeRoot: filepath.Join(host, "supervisor-runtime"), ReadOnlyRoots: []string{sdk, skill}, AllowNetwork: true},
		SessionPath: filepath.Join(host, "supervisor-runtime", "session.jsonl"), SkillRoot: skill, Identity: identity,
		Binding: PiSupervisorBinding{Root: evaluated, PreparationID: "prep", ExperimentID: "experiment", RuntimePlanPath: filepath.Join(host, "pi-runtime-plan.json")},
	}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	return plan, planPath
}
