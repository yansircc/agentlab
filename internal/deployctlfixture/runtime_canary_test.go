package deployctlfixture

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/experiment"
)

func TestBindRuntimeRejectsCanaryBeforeHostPlanOrManifest(t *testing.T) {
	piPath, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	resolved, err := filepath.EvalSymlinks(piPath)
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	value, err := ProvisionPreflight(PreflightSpec{EvaluatedRoot: filepath.Join(parent, "evaluated"), AuditRoot: filepath.Join(parent, "audit")})
	if err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(parent, "host")
	_, err = value.bindRuntime(RuntimeSpec{HostRoot: host, SkillRoot: testSkillArtifact(t), SDKRoot: filepath.Dir(filepath.Dir(resolved)), NodePath: node, Provider: "structural-test", Model: "structural-test", ThinkingPolicy: "off", CompactionPolicy: "off", ProviderCredentialEnv: "PROVIDER_TOKEN", WorkerCredentialHandle: "AGENTLAB_TEST_WORKER_TOKEN", CoderCredentialHandle: "AGENTLAB_TEST_CODER_TOKEN", SupervisorCredentialHandle: "AGENTLAB_TEST_SUPERVISOR_TOKEN"}, func(piadapter.LiveCanarySpec) (piadapter.LiveCanaryReceipt, error) {
		return piadapter.LiveCanaryReceipt{}, errors.New("final provider rejected canary")
	})
	if err == nil {
		t.Fatal("runtime preflight accepted a failed live canary")
	}
	if _, err := os.Stat(host); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed canary created Host runtime state: %v", err)
	}
	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := op.RunManifest(value.BaselineRunID); err == nil {
		t.Fatal("failed canary bound the baseline manifest")
	}
}
