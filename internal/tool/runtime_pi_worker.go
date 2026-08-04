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
	Launch              PiLaunch     `json:"launch"`
	FixtureRoot         string       `json:"fixture_root"`
	DeployctlExecutable string       `json:"deployctl_executable"`
	CandidateExecutable artifact.Ref `json:"candidate_executable"`
	WorkerInput         artifact.Ref `json:"worker_input"`
}

func (value PiWorkerLaunch) Validate() error {
	if value.Launch.Validate() != nil || len(value.Launch.AllowedExecutables) != 0 || !filepath.IsAbs(value.FixtureRoot) || !filepath.IsAbs(value.DeployctlExecutable) || !value.CandidateExecutable.Valid() || !value.WorkerInput.Valid() || overlaps(value.FixtureRoot, value.Launch.RuntimeRoot) {
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

func startPiWorker(binding Binding, operation *run.Operation, intent effect.Intent, profile PiRuntimeProfile) (any, error) {
	launch := *profile.WorkerLaunch
	prompt, err := workerPrompt(binding, intent.RunID, launch)
	if err != nil || preparePiRuntime(launch.Launch.RuntimeRoot, profile.SessionPath) != nil {
		return nil, errors.New("worker runtime is invalid")
	}
	extension, err := writeWorkerExtension(launch.Launch.RuntimeRoot)
	if err != nil {
		return nil, err
	}
	sandbox, err := coder.NewSandbox(coder.SandboxSpec{
		Workspace: launch.FixtureRoot, RuntimeRoot: launch.Launch.RuntimeRoot,
		ReadOnlyRoots: uniquePaths(append(launch.Launch.ReadOnlyRoots, profile.Identity.SDKRoot, filepath.Dir(profile.Identity.ContextFilterPath))),
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
	command, err := sandbox.Wrap(piWorkerCommand(profile.Identity, launch.Launch.NodePath, session, sandbox.RuntimeRoot(), profile.Identity.ContextFilterPath, extension, prompt))
	if err != nil {
		return nil, err
	}
	return piadapter.BeginManagedEffect(operation, intent, session, profile.Policy, command, environment, sandbox.Workspace(), nil, func(int) error {
		_, err := piadapter.Poll(operation, session)
		return err
	})
}

func workerPrompt(binding Binding, runID string, launch PiWorkerLaunch) (string, error) {
	experiment, err := binding.experiment()
	if err != nil {
		return "", err
	}
	manifest, _, err := experiment.RunManifest(runID)
	if err != nil || manifest.WorkerInput != launch.WorkerInput || run.VerifyCandidateExecutable(binding.store(), launch.CandidateExecutable, manifest.Candidate, launch.DeployctlExecutable) != nil {
		return "", errors.New("worker launch differs from manifest")
	}
	value, err := preparation.RenderWorkerInput(binding.store(), launch.WorkerInput)
	if err != nil {
		return "", err
	}
	return "You are the isolated Worker. Use only the public deployctl tools to complete the sealed task.\n\n" + value, nil
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
