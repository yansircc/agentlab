package deployctlfixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/metaaudit"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/tool"
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

func TestLoadRuntimePreflightRejectsTamperedHostLocator(t *testing.T) {
	root := t.TempDir()
	host := filepath.Join(root, "host")
	if err := os.Mkdir(host, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(host, "preflight.json"), []byte(`{"contract":"agentlab.deployctl-runtime-preflight.v1","evaluated_root":"/tmp/evaluated","audit_root":"/tmp/evaluated"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimePreflight(host); err == nil {
		t.Fatal("overlapping roots in Host locator were accepted")
	}
}

func TestRuntimeCredentialBindingsRequireThreeDistinctHostHandles(t *testing.T) {
	value := RuntimeSpec{ProviderCredentialEnv: "PROVIDER_TOKEN", WorkerCredentialHandle: "HOST_WORKER_TOKEN", CoderCredentialHandle: "HOST_CODER_TOKEN", SupervisorCredentialHandle: "HOST_SUPERVISOR_TOKEN"}
	credentials, err := value.credentials()
	if err != nil || credentials.workerEnvironment()[value.ProviderCredentialEnv] != value.WorkerCredentialHandle || credentials.coderEnvironment()[value.ProviderCredentialEnv] != value.CoderCredentialHandle || credentials.supervisorEnvironment()[value.ProviderCredentialEnv] != value.SupervisorCredentialHandle {
		t.Fatalf("runtime credential binding = %#v, %v", credentials, err)
	}
	for _, mutate := range []func(*RuntimeSpec){
		func(spec *RuntimeSpec) { spec.ProviderCredentialEnv = "" },
		func(spec *RuntimeSpec) { spec.WorkerCredentialHandle = spec.CoderCredentialHandle },
		func(spec *RuntimeSpec) { spec.SupervisorCredentialHandle = "not a valid env name" },
	} {
		invalid := value
		mutate(&invalid)
		if _, err := invalid.credentials(); err == nil {
			t.Fatalf("invalid runtime credential binding was accepted: %#v", invalid)
		}
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
	spec := RuntimeSpec{HostRoot: filepath.Join(parent, "host"), SkillRoot: testSkillArtifact(t), SDKRoot: filepath.Dir(filepath.Dir(resolved)), NodePath: node, Provider: "structural-test", Model: "structural-test", ThinkingPolicy: "off", CompactionPolicy: "off", ProviderCredentialEnv: "PROVIDER_TOKEN", WorkerCredentialHandle: "AGENTLAB_TEST_WORKER_TOKEN", CoderCredentialHandle: "AGENTLAB_TEST_CODER_TOKEN", SupervisorCredentialHandle: "AGENTLAB_TEST_SUPERVISOR_TOKEN"}
	value, err = value.bindRuntime(spec, func(canary piadapter.LiveCanarySpec) (piadapter.LiveCanaryReceipt, error) {
		if canary.NodePath != spec.NodePath || canary.SDKRoot != spec.SDKRoot || canary.Identity.Provider != spec.Provider || canary.Identity.Model != spec.Model || canary.ProviderCredentialEnv != spec.ProviderCredentialEnv || canary.CredentialHandle != spec.WorkerCredentialHandle {
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
	if value.FixtureReset == (artifact.Ref{}) || value.CoderPrepared == (artifact.Ref{}) || value.Inputs.Adapter == (artifact.Ref{}) || value.Inputs.WorkerRuntime == (artifact.Ref{}) {
		t.Fatalf("runtime preflight omitted exact inputs: %#v", value)
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	coderPrepared, err := experiment.LoadPreparedRun(store, value.CoderPrepared)
	if err != nil || coderPrepared.RunID != coderRunID || coderPrepared.Inputs.Candidate != value.Candidate {
		t.Fatalf("Coder prepared run = %#v, %v", coderPrepared, err)
	}
	host, err := tool.LoadPiRuntimeHost(filepath.Join(spec.HostRoot, "pi-runtime-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := host.Profile("baseline-worker")
	if err != nil {
		t.Fatalf("baseline Worker profile = %#v, %v", profile, err)
	}
	supervisor, err := tool.LoadPiSupervisorPlan(filepath.Join(spec.HostRoot, "supervisor-plan.json"))
	if err != nil || supervisor.Binding.Root != value.EvaluatedRoot || supervisor.Binding.PreparationID != value.PreparationID || supervisor.Binding.ExperimentID != value.ExperimentID || supervisor.Binding.RuntimePlanPath != filepath.Join(spec.HostRoot, "pi-runtime-plan.json") || supervisor.Identity != profile.Identity || supervisor.Launch.RuntimeRoot != filepath.Join(spec.HostRoot, "supervisor-runtime") || supervisor.SessionPath != filepath.Join(spec.HostRoot, "supervisor-runtime", "session.jsonl") {
		t.Fatalf("Supervisor plan = %#v, %v", supervisor, err)
	}
	if profile.WorkerLaunch == nil {
		t.Fatal("baseline Worker profile omitted launch binding")
	}
	coderProfile, err := host.Profile("coder-repair")
	if err != nil || coderProfile.CoderLaunch == nil || !verifyRuntimeCredentialIsolation(profile.WorkerLaunch.Launch, *coderProfile.CoderLaunch, supervisor.Launch) {
		t.Fatalf("runtime credential isolation = %#v / %#v / %#v, %v", profile, coderProfile, supervisor, err)
	}
	if _, err := value.SupervisorStatus(); err == nil {
		t.Fatal("runtime preflight fabricated a Supervisor process receipt")
	}
	reopened, err := LoadRuntimePreflight(spec.HostRoot)
	if err != nil || reopened.EvaluatedRoot != value.EvaluatedRoot || reopened.AuditRoot != value.AuditRoot || reopened.CoderPrepared != value.CoderPrepared || reopened.Inputs != value.Inputs || reopened.CandidateExecutable != value.CandidateExecutable {
		t.Fatalf("reopened runtime preflight = %#v, %v", reopened, err)
	}
	completion, candidate := terminalCoderCompletion(t, value)
	wrongCompletion, err := store.Put([]byte("not a terminal Coder completion"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := value.PrepareRunFromCoderCompletion("candidate-wrong", wrongCompletion); err == nil {
		t.Fatal("Host producer accepted a non-terminal Coder completion")
	}
	baselinePreparedRef, err := value.PrepareBaselineRun("baseline-repeat")
	if err != nil {
		t.Fatal(err)
	}
	baselinePrepared, err := experiment.LoadPreparedRun(store, baselinePreparedRef)
	if err != nil || baselinePrepared.RunID != "baseline-repeat" || baselinePrepared.Inputs.Candidate != value.Candidate || baselinePrepared.Inputs.WorkerRuntime == value.Inputs.WorkerRuntime || baselinePrepared.Inputs.Harness != value.Inputs.Harness || baselinePrepared.Inputs.Trial != value.Inputs.Trial || baselinePrepared.Inputs.Adapter != value.LiveCanary || baselinePrepared.Inputs.OracleSet != value.Inputs.OracleSet || baselinePrepared.Inputs.Fixture != value.Inputs.Fixture || baselinePrepared.Inputs.EvidencePolicy != value.Inputs.EvidencePolicy || baselinePrepared.Inputs.StopPolicy != value.Inputs.StopPolicy || baselinePrepared.Inputs.Environment != value.Inputs.Environment {
		t.Fatalf("baseline repetition inputs = %#v, %v", baselinePrepared, err)
	}
	if _, err := value.PrepareBaselineRun("baseline-repeat"); err == nil {
		t.Fatal("Host producer rebound a baseline repetition")
	}
	preparedRef, err := value.PrepareRunFromCoderCompletion("candidate-worker", completion)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := experiment.LoadPreparedRun(store, preparedRef)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.RunID != "candidate-worker" || prepared.Inputs.Candidate != candidate || prepared.Inputs.Harness != value.Inputs.Harness || prepared.Inputs.Trial != value.Inputs.Trial || prepared.Inputs.Adapter != value.LiveCanary || prepared.Inputs.OracleSet != value.Inputs.OracleSet || prepared.Inputs.Fixture != value.Inputs.Fixture || prepared.Inputs.EvidencePolicy != value.Inputs.EvidencePolicy || prepared.Inputs.StopPolicy != value.Inputs.StopPolicy || prepared.Inputs.Environment != value.Inputs.Environment {
		t.Fatalf("prepared run changed a stable input: %#v", prepared)
	}
	resetData, err := store.Read(prepared.Inputs.FixtureReset)
	if err != nil {
		t.Fatal(err)
	}
	var reset experiment.FixtureResetProof
	if err := json.Unmarshal(resetData, &reset); err != nil || reset.RunID != prepared.RunID || reset.Fixture != prepared.Inputs.Fixture {
		t.Fatalf("prepared reset = %#v, %v", reset, err)
	}
	runtimeBinding, err := loadRuntimeBinding(store, prepared.Inputs.WorkerRuntime)
	if err != nil || runtimeBinding.Adapter != value.LiveCanary || runtimeBinding.CandidateExecutable == value.CandidateExecutable || runtimeBinding.WorkerProfile != candidateWorkerProfileRef("candidate-worker") {
		t.Fatalf("prepared runtime binding = %#v, %v", runtimeBinding, err)
	}
	host, err = tool.LoadPiRuntimeHost(filepath.Join(spec.HostRoot, "pi-runtime-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	candidateProfile, err := host.PreparedWorker(runtimeBinding.WorkerProfile)
	if err != nil || candidateProfile.RunID != prepared.RunID || candidateProfile.WorkerLaunch.FixtureRoot == value.Fixture.Root() || candidateProfile.WorkerLaunch.CandidateExecutable != runtimeBinding.CandidateExecutable || candidateProfile.WorkerRuntime != prepared.Inputs.WorkerRuntime || candidateProfile.Forked != nil {
		t.Fatalf("candidate Host prepared runtime = %#v, %v", candidateProfile, err)
	}
	if _, err := host.Profile(runtimeBinding.WorkerProfile); err == nil {
		t.Fatal("prepared Worker runtime was exposed as a fresh static profile")
	}
	if err := VerifyBuild(store, runtimeBinding.CandidateExecutable, prepared.Inputs.Candidate, candidateProfile.WorkerLaunch.DeployctlExecutable); err != nil {
		t.Fatalf("candidate executable differs from prepared snapshot: %v", err)
	}
	heldoutRef, err := value.VerifyHeldoutPreparedRun(preparedRef)
	if err != nil {
		t.Fatal(err)
	}
	heldoutData, err := store.Read(heldoutRef)
	if err != nil {
		t.Fatal(err)
	}
	var heldout HeldoutVerification
	if err := json.Unmarshal(heldoutData, &heldout); err != nil || heldout.Contract != heldoutVerificationContract || heldout.Prepared != preparedRef || heldout.Candidate != prepared.Inputs.Candidate || !heldout.TerminalSuccess || !heldout.Oracle.Pass() || heldout.Oracle.DefaultTargetReadCount != 0 {
		t.Fatalf("held-out verification = %#v, %v", heldout, err)
	}
	if err := VerifyHeldoutArtifact(store, heldoutRef, prepared.Inputs.Candidate); err != nil {
		t.Fatalf("exact held-out verification was rejected: %v", err)
	}
	if err := VerifyHeldoutArtifact(store, heldoutRef, value.Candidate); err == nil {
		t.Fatal("held-out verification was reused for another candidate")
	}
	heldout.TerminalSuccess = false
	invalidData, err := json.Marshal(heldout)
	if err != nil {
		t.Fatal(err)
	}
	invalidHeldout, err := store.PutCanonicalJSON(invalidData)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyHeldoutArtifact(store, invalidHeldout, prepared.Inputs.Candidate); err == nil {
		t.Fatal("failed held-out verification was accepted")
	}
	audit, err := metaaudit.Open(value.AuditRoot, value.AuditID)
	if err != nil || audit.MarkIntervened() != nil {
		t.Fatalf("advance audit lifecycle: %v", err)
	}
	if _, err := LoadRuntimePreflight(spec.HostRoot); err != nil {
		t.Fatalf("runtime preflight did not survive later audit state: %v", err)
	}
}

func terminalCoderCompletion(t *testing.T, value Preflight) (artifact.Ref, artifact.Ref) {
	t.Helper()
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	candidateFiles := BaselineSource()
	for index := range candidateFiles {
		if candidateFiles[index].Path == "deploy.go" {
			candidateFiles[index].Content = bytes.Replace(candidateFiles[index].Content, []byte("actual, err := defaultTarget(root, catalog)\n\tif err != nil { return err }"), []byte("actual := target"), 1)
		}
	}
	candidate, err := source.Build(store, candidateFiles)
	if err != nil || candidate == value.Candidate {
		t.Fatalf("candidate snapshot = %#v, %v", candidate, err)
	}
	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.BindPreparedRun(coderRunID, experiment.NewFreshOrigin(), value.CoderPrepared); err != nil {
		t.Fatal(err)
	}
	host, err := tool.LoadPiRuntimeHost(value.runtimePlanPath)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := host.Profile("coder-repair")
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := store.Put([]byte("terminal Coder handoff"))
	if err != nil {
		t.Fatal(err)
	}
	coderProfile := run.CoderProfile{Handoff: handoff, SourceSnapshot: profile.CoderSourceSnapshot, CandidateWorkspace: profile.CoderWorkspaceReceipt, CapabilityProfile: profile.CoderCapabilityProfile}
	payload, err := run.EncodeStartPayload(effect.CoderStart, run.StartPayload{Coder: &coderProfile})
	if err != nil {
		t.Fatal(err)
	}
	payloadRef, err := store.Put(payload)
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "terminal-coder", RunID: coderRunID, Kind: effect.CoderStart, Payload: payloadRef}
	coder, err := run.Open(value.EvaluatedRoot, value.ExperimentID, coderRunID)
	if err != nil {
		t.Fatal(err)
	}
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true}
	if _, err := coder.BeginManagedAttachedEffect(intent, run.ManagedAttachedSpec{
		Adapter: "test", Policy: policy, Capabilities: run.RequiredAdapterCapabilities(), Command: []string{"/bin/sh", "-c", "exit 0"}, Environment: []string{"PATH=/usr/bin:/bin"}, WorkingDirectory: value.EvaluatedRoot,
		Ready: func() (string, []byte, error) { return "terminal-coder-session", []byte("cursor"), nil },
		Coder: &coderProfile,
		Finalize: func(code int) error {
			if code != 0 {
				return errors.New("test Coder exited unsuccessfully")
			}
			_, err := coder.RecordCoderCompletion(candidate)
			return err
		},
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		receipt, _, err := coder.CoderCompletionReceipt()
		if err == nil {
			return receipt, candidate
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("terminal Coder completion was not admitted")
	return artifact.Ref{}, artifact.Ref{}
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
