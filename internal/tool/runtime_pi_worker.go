package tool

import (
	"errors"
	"os"
	"path/filepath"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/coder"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
)

// PiWorkerLaunch grants only the public fixture CLI to an owned Worker.
// It deliberately has no shell or generic executable capability.
type PiWorkerLaunch struct {
	Launch              PiLaunch       `json:"launch"`
	FixtureRoot         string         `json:"fixture_root"`
	DeployctlExecutable string         `json:"deployctl_executable"`
	CandidateExecutable artifact.Ref   `json:"candidate_executable"`
	WorkerInput         artifact.Ref   `json:"worker_input"`
	HostOracle          HostOracleKind `json:"host_oracle,omitempty"`
}

func (value PiWorkerLaunch) Validate() error {
	if value.Launch.Validate() != nil || len(value.Launch.AllowedExecutables) != 0 || !filepath.IsAbs(value.FixtureRoot) || !filepath.IsAbs(value.DeployctlExecutable) || !value.CandidateExecutable.Valid() || !value.WorkerInput.Valid() || !value.HostOracle.Valid() || overlaps(value.FixtureRoot, value.Launch.RuntimeRoot) {
		return errors.New("worker launch is invalid")
	}
	for _, root := range value.Launch.ReadOnlyRoots {
		if overlaps(value.FixtureRoot, root) || overlaps(value.Launch.RuntimeRoot, root) {
			return errors.New("worker launch roots overlap")
		}
	}
	return nil
}

func (value PiWorkerLaunch) clone() PiWorkerLaunch {
	value.Launch = value.Launch.clone()
	return value
}

func startPiWorker(binding Binding, operation *run.Operation, intent effect.Intent, profile PiRuntimeProfile, hostOracle hostWorkerOracle) (any, error) {
	launch := *profile.WorkerLaunch
	if launch.HostOracle != HostOracleNone && hostOracle == nil {
		return nil, errors.New("Host Worker oracle is unavailable")
	}
	prompt, err := workerPrompt(binding, intent.RunID, launch)
	if err != nil || prepareWorkerRuntime(launch.Launch.RuntimeRoot, profile.SessionPath, profile.resumeExistingSession) != nil {
		return nil, errors.New("worker runtime is invalid")
	}
	extension, err := writeWorkerExtension(launch.Launch.RuntimeRoot)
	if err != nil {
		return nil, err
	}
	identity, err := canonicalPiIdentity(profile.Identity)
	if err != nil {
		return nil, err
	}
	sandbox, err := coder.NewSandbox(coder.SandboxSpec{
		Workspace: launch.FixtureRoot, RuntimeRoot: launch.Launch.RuntimeRoot,
		ReadOnlyRoots: uniquePaths(append(launch.Launch.ReadOnlyRoots, identity.SDKRoot, filepath.Dir(profile.Identity.ContextFilterPath))),
		Executables:   uniquePaths([]string{launch.Launch.NodePath, launch.DeployctlExecutable}), AllowNetwork: launch.Launch.AllowNetwork,
	})
	if err != nil {
		return nil, err
	}
	session, err := runtimeSession(sandbox.RuntimeRoot(), profile.SessionPath)
	if err != nil {
		return nil, err
	}
	environment, err := launch.Launch.environment(sandbox.RuntimeRoot())
	if err != nil {
		return nil, err
	}
	environment = append(environment, "AGENTLAB_CONTEXT_FILTER_ONLY=1", "AGENTLAB_WORKER_FIXTURE="+sandbox.Workspace(), "AGENTLAB_WORKER_DEPLOYCTL="+launch.DeployctlExecutable)
	command, err := sandbox.Wrap(piWorkerCommand(identity, launch.Launch.NodePath, session, sandbox.RuntimeRoot(), profile.Identity.ContextFilterPath, extension, prompt))
	if err != nil {
		return nil, err
	}
	return piadapter.BeginManagedEffect(operation, intent, session, profile.Policy, command, environment, sandbox.Workspace(), nil, finalizePiWorker(operation, session, launch.HostOracle, intent.RunID, profile.Identity.AdapterDigest, hostOracle))
}

// finalizePiWorker records the complete public session before the Host takes
// its objective fixture observation. The managed runner invokes this callback
// before it writes the terminal event, so the Host evidence is the single
// oracle fact for a successfully exited, non-stopped Worker run.
func finalizePiWorker(operation *run.Operation, session string, kind HostOracleKind, runID, binaryDigest string, hostOracle hostWorkerOracle) func(int) error {
	return func(code int) error {
		polled, err := piadapter.Poll(operation, session)
		if err != nil {
			return err
		}
		if code != 0 || polled.Stopped || kind == HostOracleNone {
			return nil
		}
		if hostOracle == nil {
			return errors.New("Host Worker oracle is unavailable")
		}
		return hostOracle(kind, runID, binaryDigest)
	}
}

func prepareWorkerRuntime(root, session string, resume bool) error {
	if !resume {
		return preparePiRuntime(root, session)
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(session) || !inside(root, session) {
		return errors.New("forked Worker runtime path is invalid")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].IsDir() || entries[0].Name() != filepath.Base(session) {
		return errors.New("forked Worker runtime root differs from receipt")
	}
	info, err := os.Lstat(session)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("forked Worker session is invalid")
	}
	return nil
}

func workerPrompt(binding Binding, runID string, launch PiWorkerLaunch) (string, error) {
	experimentOp, err := binding.experiment()
	if err != nil {
		return "", err
	}
	manifest, _, err := experimentOp.RunManifest(runID)
	if err != nil || manifest.WorkerInput != launch.WorkerInput || run.VerifyCandidateExecutable(binding.store(), launch.CandidateExecutable, manifest.Candidate, launch.DeployctlExecutable) != nil {
		return "", errors.New("worker launch differs from manifest")
	}
	value, err := preparation.RenderWorkerInput(binding.store(), launch.WorkerInput)
	if err != nil {
		return "", err
	}
	prompt := "You are the isolated Worker. Use only the public deployctl tools to complete the sealed task.\n\n" + value
	if origin, ok := manifest.Origin.Splice(); ok && origin.Intervention != nil {
		intervention, err := experimentOp.Intervention(*origin.Intervention)
		if err != nil {
			return "", errors.New("splice intervention is invalid")
		}
		// The artifact text is the entire new model-visible fact. The static
		// delimiter only makes its role explicit; no provider input is merged.
		prompt += "\n\nIntervention:\n" + intervention.Text
	}
	return prompt, nil
}

func writeWorkerExtension(root string) (string, error) {
	path := filepath.Join(root, "agentlab-worker-tools.ts")
	if err := os.WriteFile(path, []byte(workerExtensionSource), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func piWorkerCommand(identity piadapter.IdentityConfig, node, session, runtimeRoot, contextExtension, workerExtension, prompt string) []string {
	return []string{node, filepath.Join(identity.SDKRoot, "dist", "cli.js"), "--session", session, "--session-dir", runtimeRoot, "--provider", identity.Provider, "--model", identity.Model, "--thinking", identity.ThinkingPolicy, "--no-extensions", "--extension", contextExtension, "--extension", workerExtension, "--no-builtin-tools", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--tools", "deployctl_help,deployctl_deploy,deployctl_status,deployctl_receipt", "--print", prompt}
}
