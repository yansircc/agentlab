package deployctlfixture

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/metaaudit"
	"github.com/yansircc/agentlab/internal/preparation"
)

func TestProvisionPreflightBuildsDisjointHostAssembly(t *testing.T) {
	parent := t.TempDir()
	value, err := ProvisionPreflight(PreflightSpec{EvaluatedRoot: filepath.Join(parent, "evaluated"), AuditRoot: filepath.Join(parent, "audit")})
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Verify(); err == nil {
		t.Fatal("provisioning was misclassified as an exact runtime preflight")
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	input, err := preparation.RenderWorkerInput(store, value.WorkerInput)
	if err != nil || bytes.Contains([]byte(input), privateFact) || bytes.Contains([]byte(input), []byte("defaultTarget(")) {
		t.Fatalf("public input = %q, %v", input, err)
	}
	if _, err := store.Read(value.GroundTruth); err == nil {
		t.Fatal("evaluated root read the private ground truth")
	}
	auditStore := artifact.NewStore(filepath.Join(value.AuditRoot, "artifacts"))
	if _, err := auditStore.Read(value.Candidate); err == nil {
		t.Fatal("audit root read the baseline candidate")
	}
	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := op.RunManifest(value.BaselineRunID); err == nil {
		t.Fatal("provisioning bound a run before exact Pi identity was available")
	}
	audit, err := metaaudit.Open(value.AuditRoot, value.AuditID)
	if err != nil {
		t.Fatal(err)
	}
	status, err := audit.Status()
	if err != nil || status.Sealed || status.Intervened || len(status.FindingIDs) != 0 {
		t.Fatalf("audit status = %#v, %v", status, err)
	}
	if oracle, err := value.Fixture.Oracle("staging", "release-a"); err != nil || oracle.Pass() || oracle.DefaultTargetReadCount != 0 || len(oracle.WriteSet) != 0 {
		t.Fatalf("preflight mutated the fixture: %#v, %v", oracle, err)
	}
}

func TestProvisionPreflightRejectsExistingOrOverlappingRoots(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "evaluated")
	if _, err := ProvisionPreflight(PreflightSpec{EvaluatedRoot: root, AuditRoot: filepath.Join(parent, "audit")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionPreflight(PreflightSpec{EvaluatedRoot: root, AuditRoot: filepath.Join(parent, "next-audit")}); err == nil {
		t.Fatal("existing evaluated root was reused")
	}
	if _, err := ProvisionPreflight(PreflightSpec{EvaluatedRoot: filepath.Join(parent, "one"), AuditRoot: filepath.Join(parent, "one", "audit")}); err == nil {
		t.Fatal("overlapping capability roots were accepted")
	}
}

func TestBindRuntimeBuildsAHostPrivateExactProfilePlan(t *testing.T) {
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
	spec := RuntimeSpec{HostRoot: filepath.Join(parent, "host"), SkillRoot: testSkillArtifact(t), SDKRoot: filepath.Dir(filepath.Dir(resolved)), NodePath: node, Provider: "structural-test", Model: "structural-test", ThinkingPolicy: "off", CompactionPolicy: "off"}
	value, err = value.bindRuntime(spec, func(canary piadapter.LiveCanarySpec) (piadapter.LiveCanaryReceipt, error) {
		if canary.NodePath != spec.NodePath || canary.SDKRoot != spec.SDKRoot || canary.Identity.Provider != spec.Provider || canary.Identity.Model != spec.Model {
			t.Fatalf("canary binding = %#v", canary)
		}
		return piadapter.LiveCanaryReceipt{Contract: piadapter.LiveCanaryContract, PublicSuffixExcluded: true, PrivateThinkingExcluded: true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Verify(); err != nil {
		t.Fatal(err)
	}
	if value.FixtureReset == (artifact.Ref{}) || value.Inputs.Adapter == (artifact.Ref{}) || value.Inputs.WorkerRuntime == (artifact.Ref{}) {
		t.Fatalf("runtime preflight omitted exact inputs: %#v", value)
	}
}

func testSkillArtifact(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source is unavailable")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	skill := filepath.Join(t.TempDir(), "skill")
	if err := os.MkdirAll(filepath.Join(skill, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "extension.ts"} {
		data, err := os.ReadFile(filepath.Join(repo, "skills", "agentlab", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(skill, name), data, 0o600); err != nil {
			t.Fatalf("copy %s: %v", name, err)
		}
	}
	command := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(skill, "bin", "agentlab"), "./cmd/agentlab")
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build bundled artifact: %s: %v", output, err)
	}
	return skill
}
