package tool

import (
	"errors"
	"path/filepath"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/coder"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
)

const piRuntimePlanContract = "agentlab.pi-runtime-plan.v2"

// PiRuntimeProfile is Host-private runtime authority. It is registered before
// the Supervisor starts; only Ref crosses the four-tool boundary.
type PiRuntimeProfile struct {
	Ref                    string                   `json:"ref"`
	ExperimentID           string                   `json:"experiment_id"`
	RunID                  string                   `json:"run_id"`
	Role                   effect.Kind              `json:"role"`
	SessionPath            string                   `json:"session_path"`
	Identity               piadapter.IdentityConfig `json:"identity"`
	Policy                 run.StopPolicy           `json:"policy"`
	WorkerLaunch           *PiWorkerLaunch          `json:"worker_launch,omitempty"`
	CoderSourceSnapshot    artifact.Ref             `json:"coder_source_snapshot,omitempty"`
	CoderWorkspaceReceipt  artifact.Ref             `json:"coder_workspace_receipt,omitempty"`
	CoderCapabilityProfile artifact.Ref             `json:"coder_capability_profile,omitempty"`
	CoderWorkspace         string                   `json:"coder_workspace,omitempty"`
	CoderLaunch            *PiLaunch                `json:"coder_launch,omitempty"`
	resumeExistingSession  bool
}

type PiRuntimeHost struct {
	profiles        map[string]PiRuntimeProfile
	preparedWorkers map[string]PiPreparedWorkerRuntime
	planPath        string
	hostOracle      hostWorkerOracle
}

// PiPollResult exposes only the Host-authored Coder completion receipt after
// its terminal ledger fact exists. It never exposes session bytes or paths.
type PiPollResult struct {
	piadapter.PollResult
	CoderCompletion *artifact.Ref                  `json:"coder_completion,omitempty"`
	RunStatus       *run.OperationStatusProjection `json:"run_status,omitempty"`
}

func NewPiRuntimeHost(profiles []PiRuntimeProfile) (*PiRuntimeHost, error) {
	return newPiRuntimeHost(profiles, nil)
}

func newPiRuntimeHost(profiles []PiRuntimeProfile, preparedWorkers []PiPreparedWorkerRuntime) (*PiRuntimeHost, error) {
	result := &PiRuntimeHost{profiles: make(map[string]PiRuntimeProfile, len(profiles)), preparedWorkers: make(map[string]PiPreparedWorkerRuntime, len(preparedWorkers))}
	sessions := map[string]bool{}
	var workspaces, runtimes []string
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil || result.profiles[profile.Ref].Ref != "" || sessions[profile.SessionPath] || profileOverlaps(profile, workspaces, runtimes) {
			return nil, errors.New("Pi runtime profile is invalid")
		}
		copy := profile
		if profile.CoderLaunch != nil {
			launch := profile.CoderLaunch.clone()
			copy.CoderLaunch = &launch
		}
		if profile.WorkerLaunch != nil {
			launch := profile.WorkerLaunch.clone()
			copy.WorkerLaunch = &launch
		}
		result.profiles[profile.Ref] = copy
		sessions[profile.SessionPath] = true
		if profile.CoderWorkspace != "" {
			workspaces = append(workspaces, profile.CoderWorkspace)
		}
		if profile.CoderLaunch != nil {
			runtimes = append(runtimes, profile.CoderLaunch.RuntimeRoot)
		}
		if profile.WorkerLaunch != nil {
			workspaces = append(workspaces, profile.WorkerLaunch.FixtureRoot)
			runtimes = append(runtimes, profile.WorkerLaunch.Launch.RuntimeRoot)
		}
	}
	for _, template := range preparedWorkers {
		if err := template.Validate(); err != nil || result.profiles[template.Ref].Ref != "" || result.preparedWorkers[template.Ref].Ref != "" || sessions[template.FreshSessionPath] || profileOverlaps(PiRuntimeProfile{WorkerLaunch: &template.WorkerLaunch}, workspaces, runtimes) {
			return nil, errors.New("prepared Pi Worker runtime is invalid")
		}
		copy := template.clone()
		result.preparedWorkers[template.Ref] = copy
		sessions[template.FreshSessionPath] = true
		workspaces = append(workspaces, template.WorkerLaunch.FixtureRoot)
		runtimes = append(runtimes, template.WorkerLaunch.Launch.RuntimeRoot)
	}
	return result, nil
}

func DecodePiRuntimeHost(data []byte) (*PiRuntimeHost, error) {
	plan, err := decodePiRuntimePlan(data)
	if err != nil {
		return nil, err
	}
	return newPiRuntimeHost(plan.Profiles, plan.PreparedWorkers)
}

func (value PiRuntimeProfile) Validate() error {
	if value.Ref == "" || value.ExperimentID == "" || value.RunID == "" || (value.Role != effect.WorkerStart && value.Role != effect.CoderStart) || !filepath.IsAbs(value.SessionPath) || !filepath.IsAbs(value.Identity.SDKRoot) || !filepath.IsAbs(value.Identity.ContextFilterPath) || value.Policy.Validate() != nil {
		return errors.New("Pi runtime profile is invalid")
	}
	if value.Role == effect.WorkerStart && (value.hasCoderTemplate() || value.WorkerLaunch == nil || value.WorkerLaunch.Validate() != nil || !value.Policy.OwnsWorkerProcess || value.Policy.KillOnHardIdle || !inside(value.WorkerLaunch.Launch.RuntimeRoot, value.SessionPath) || overlaps(value.WorkerLaunch.FixtureRoot, value.Identity.SDKRoot) || overlaps(value.WorkerLaunch.Launch.RuntimeRoot, value.Identity.SDKRoot)) {
		return errors.New("worker runtime profile is invalid")
	}
	if value.Role == effect.CoderStart && (value.WorkerLaunch != nil || !value.CoderSourceSnapshot.Valid() || !value.CoderWorkspaceReceipt.Valid() || !value.CoderCapabilityProfile.Valid() || !filepath.IsAbs(value.CoderWorkspace) || value.CoderLaunch == nil || value.CoderLaunch.Validate() != nil || !value.Policy.OwnsWorkerProcess || value.Policy.KillOnHardIdle || !inside(value.CoderLaunch.RuntimeRoot, value.SessionPath) || overlaps(value.CoderWorkspace, value.CoderLaunch.RuntimeRoot) || overlaps(value.CoderWorkspace, value.Identity.SDKRoot) || overlaps(value.CoderLaunch.RuntimeRoot, value.Identity.SDKRoot)) {
		return errors.New("coder runtime profile is invalid")
	}
	return nil
}

func (value PiRuntimeProfile) hasCoderTemplate() bool {
	return value.CoderSourceSnapshot != (artifact.Ref{}) || value.CoderWorkspaceReceipt != (artifact.Ref{}) || value.CoderCapabilityProfile != (artifact.Ref{}) || value.CoderWorkspace != "" || value.CoderLaunch != nil
}

func profileOverlaps(profile PiRuntimeProfile, workspaces, runtimes []string) bool {
	workspace, runtime := profileRoots(profile)
	if workspace == "" || runtime == "" {
		return false
	}
	for _, path := range append(append([]string{}, workspaces...), runtimes...) {
		if overlaps(workspace, path) || overlaps(runtime, path) {
			return true
		}
	}
	return false
}

func profileRoots(profile PiRuntimeProfile) (string, string) {
	if profile.WorkerLaunch != nil {
		return profile.WorkerLaunch.FixtureRoot, profile.WorkerLaunch.Launch.RuntimeRoot
	}
	if profile.CoderLaunch != nil {
		return profile.CoderWorkspace, profile.CoderLaunch.RuntimeRoot
	}
	return "", ""
}

func (h *PiRuntimeHost) Start(binding Binding, intent effect.Intent, ref string) (any, error) {
	profile, err := h.activeWorkerProfile(binding, intent.RunID, ref)
	if err == nil {
		if intent.Kind != effect.WorkerStart {
			return nil, errors.New("Pi start profile differs from intent")
		}
		if _, err := piadapter.VerifyRuntimeIdentity(profile.Identity); err != nil {
			return nil, errors.New("Pi runtime identity differs from Host binding")
		}
		op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
		if err != nil {
			return nil, err
		}
		return startPiWorker(binding, op, intent, profile, h.hostOracle)
	}
	profile, err = h.profile(binding, intent.RunID, ref)
	if err != nil || intent.Kind != profile.Role {
		return nil, errors.New("Pi start profile differs from intent")
	}
	if _, err := piadapter.VerifyRuntimeIdentity(profile.Identity); err != nil {
		return nil, errors.New("Pi runtime identity differs from Host binding")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	if intent.Kind == effect.CoderStart {
		profileReceipt, err := op.CoderProfile(intent)
		if err != nil || profileReceipt.SourceSnapshot != profile.CoderSourceSnapshot || profileReceipt.CandidateWorkspace != profile.CoderWorkspaceReceipt || profileReceipt.CapabilityProfile != profile.CoderCapabilityProfile {
			return nil, errors.New("coder profile differs from Host binding")
		}
		if _, err := coder.Open(binding.store(), profileReceipt.CandidateWorkspace, profileReceipt.SourceSnapshot, profile.CoderWorkspace); err != nil {
			return nil, errors.New("coder workspace differs from Host capability")
		}
		return startPiCoder(binding, op, intent, profile, profileReceipt)
	}
	return nil, errors.New("Pi start role is invalid")
}

func (h *PiRuntimeHost) Poll(binding Binding, runID, ref string) (any, error) {
	profile, workerErr := h.activeWorkerProfile(binding, runID, ref)
	if workerErr != nil {
		var err error
		profile, err = h.profile(binding, runID, ref)
		if err != nil {
			return nil, err
		}
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, runID)
	if err != nil {
		return nil, err
	}
	if profile.Role == effect.CoderStart {
		if receipt, _, err := op.CoderCompletionReceipt(); err == nil {
			return PiPollResult{CoderCompletion: &receipt}, nil
		}
	}
	polled, err := piadapter.Poll(op, profile.SessionPath)
	if err != nil {
		return nil, err
	}
	// Project the run's process liveness so the Supervisor sees an exited
	// Worker and stops polling instead of waiting for an event that never
	// comes.
	status, err := op.ProjectStatus(processidentity.SystemProber{})
	if err != nil {
		return nil, err
	}
	return PiPollResult{PollResult: polled, RunStatus: &status}, nil
}

func (h *PiRuntimeHost) Checkpoint(binding Binding, intent effect.Intent, ref string) (any, error) {
	profile, err := h.activeWorkerProfile(binding, intent.RunID, ref)
	if err != nil || intent.Kind != effect.Checkpoint {
		return nil, errors.New("Pi checkpoint profile differs from intent")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	return piadapter.CheckpointEffect(op, intent, piadapter.CheckpointEffectSpec{SDKRoot: profile.Identity.SDKRoot, ContextFilterPath: profile.Identity.ContextFilterPath, SessionPath: profile.SessionPath})
}

func (h *PiRuntimeHost) Fork(binding Binding, intent effect.Intent, ref, childRun string) (any, error) {
	profile, err := h.activeWorkerProfile(binding, intent.RunID, ref)
	if err != nil || h.planPath == "" || intent.Kind != effect.Fork || childRun == "" {
		return nil, errors.New("Pi fork profile differs from intent")
	}
	template, err := h.preparedWorkerForRun(binding, childRun)
	if err != nil {
		return nil, errors.New("Pi fork child runtime is absent")
	}
	manifest, manifestRef, err := preparedWorkerManifest(binding, template)
	if err != nil {
		return nil, err
	}
	origin, ok := manifest.Origin.Splice()
	if !ok || origin.ParentRun != intent.RunID {
		return nil, errors.New("Pi fork child origin differs from parent")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	payloadData, err := op.ReadEffectPayload(intent)
	if err != nil {
		return nil, err
	}
	payload, err := piadapter.DecodeForkPayload(payloadData)
	if err != nil || payload.ChildRun != childRun || payload.Checkpoint != origin.RuntimeCheckpoint {
		return nil, errors.New("Pi fork intent differs from child origin")
	}
	if template.Forked != nil && (template.Forked.ParentRun != intent.RunID || template.Forked.ForkReceipt.IntentID != intent.ID || template.Forked.Forked.ExpectedCheckpoint != payload.Checkpoint) {
		return nil, errors.New("Pi fork child is already bound to another receipt")
	}
	result, err := piadapter.Fork(op, intent, piadapter.ForkSpec{SDKRoot: profile.Identity.SDKRoot, ContextFilterPath: profile.Identity.ContextFilterPath, ParentSession: profile.SessionPath, ChildSessionDir: template.WorkerLaunch.Launch.RuntimeRoot})
	if err != nil {
		return nil, err
	}
	forked := PiForkedWorkerBinding{ParentRun: intent.RunID, ParentRuntimeRef: ref, ChildManifest: manifestRef, ForkReceipt: result.Receipt, Forked: result.Forked}
	if err := h.bindForkedPreparedWorker(template.Ref, forked); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *PiRuntimeHost) profile(binding Binding, runID, ref string) (PiRuntimeProfile, error) {
	if h == nil || binding.ExperimentID == "" || ref == "" {
		return PiRuntimeProfile{}, errors.New("Pi runtime profile is absent")
	}
	profile := h.profiles[ref]
	if profile.Ref == "" || profile.ExperimentID != binding.ExperimentID || profile.RunID != runID {
		return PiRuntimeProfile{}, errors.New("Pi runtime profile is outside Host binding")
	}
	return profile, nil
}

func (h *PiRuntimeHost) preparedWorker(binding Binding, runID, ref string) (PiPreparedWorkerRuntime, error) {
	if h == nil || binding.ExperimentID == "" || ref == "" {
		return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is absent")
	}
	profile := h.preparedWorkers[ref]
	if profile.Ref == "" || profile.ExperimentID != binding.ExperimentID || profile.RunID != runID {
		return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is outside Host binding")
	}
	return profile.clone(), nil
}

func (h *PiRuntimeHost) preparedWorkerForRun(binding Binding, runID string) (PiPreparedWorkerRuntime, error) {
	if h == nil || binding.ExperimentID == "" || runID == "" {
		return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is absent")
	}
	var result PiPreparedWorkerRuntime
	for _, profile := range h.preparedWorkers {
		if profile.ExperimentID != binding.ExperimentID || profile.RunID != runID {
			continue
		}
		if result.Ref != "" {
			return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is ambiguous")
		}
		result = profile
	}
	if result.Ref == "" {
		return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is absent")
	}
	return result.clone(), nil
}

func (h *PiRuntimeHost) PreparedWorker(ref string) (PiPreparedWorkerRuntime, error) {
	if h == nil || ref == "" {
		return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is absent")
	}
	value := h.preparedWorkers[ref]
	if value.Ref == "" {
		return PiPreparedWorkerRuntime{}, errors.New("prepared Pi Worker runtime is absent")
	}
	return value.clone(), nil
}

func (h *PiRuntimeHost) bindForkedPreparedWorker(ref string, forked PiForkedWorkerBinding) error {
	if h == nil || h.planPath == "" {
		return errors.New("Pi runtime plan is unavailable")
	}
	if err := BindPiForkedWorkerRuntime(h.planPath, ref, forked); err != nil {
		return err
	}
	profile := h.preparedWorkers[ref]
	if profile.Ref == "" {
		return errors.New("prepared Pi Worker runtime is absent")
	}
	copy := forked
	profile.Forked = &copy
	h.preparedWorkers[ref] = profile
	return nil
}
