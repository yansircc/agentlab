package tool

import (
	"errors"
	"path/filepath"
	"reflect"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/coder"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const piRuntimePlanContract = "agentlab.pi-runtime-plan.v1"

// PiRuntimeProfile is Host-private runtime authority. It is registered before
// the Supervisor starts; only Ref crosses the four-tool boundary.
type PiRuntimeProfile struct {
	Ref             string                   `json:"ref"`
	ExperimentID    string                   `json:"experiment_id"`
	RunID           string                   `json:"run_id"`
	Role            effect.Kind              `json:"role"`
	SessionPath     string                   `json:"session_path"`
	Identity        piadapter.IdentityConfig `json:"identity"`
	ChildSessionDir string                   `json:"child_session_dir,omitempty"`
	Policy          run.StopPolicy           `json:"policy"`
	WorkerLaunch    *PiWorkerLaunch          `json:"worker_launch,omitempty"`
	Coder           *run.CoderProfile        `json:"coder,omitempty"`
	CoderWorkspace  string                   `json:"coder_workspace,omitempty"`
	CoderLaunch     *PiLaunch                `json:"coder_launch,omitempty"`
}

type PiRuntimeHost struct{ profiles map[string]PiRuntimeProfile }

func NewPiRuntimeHost(profiles []PiRuntimeProfile) (*PiRuntimeHost, error) {
	result := &PiRuntimeHost{profiles: make(map[string]PiRuntimeProfile, len(profiles))}
	sessions := map[string]bool{}
	var workspaces, runtimes []string
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil || result.profiles[profile.Ref].Ref != "" || sessions[profile.SessionPath] || profileOverlaps(profile, workspaces, runtimes) {
			return nil, errors.New("Pi runtime profile is invalid")
		}
		copy := profile
		if profile.Coder != nil {
			coder := *profile.Coder
			copy.Coder = &coder
		}
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
	return result, nil
}

func DecodePiRuntimeHost(data []byte) (*PiRuntimeHost, error) {
	var plan struct {
		Contract string             `json:"contract"`
		Profiles []PiRuntimeProfile `json:"profiles"`
	}
	if strictjson.Decode(data, &plan) != nil || plan.Contract != piRuntimePlanContract || len(plan.Profiles) == 0 || len(plan.Profiles) > 1000 {
		return nil, errors.New("Pi runtime plan is invalid")
	}
	return NewPiRuntimeHost(plan.Profiles)
}

func (value PiRuntimeProfile) Validate() error {
	if value.Ref == "" || value.ExperimentID == "" || value.RunID == "" || (value.Role != effect.WorkerStart && value.Role != effect.CoderStart) || !filepath.IsAbs(value.SessionPath) || !filepath.IsAbs(value.Identity.SDKRoot) || !filepath.IsAbs(value.Identity.ContextFilterPath) || value.Policy.Validate() != nil {
		return errors.New("Pi runtime profile is invalid")
	}
	if value.Role == effect.WorkerStart && (value.Coder != nil || value.CoderWorkspace != "" || value.CoderLaunch != nil || value.WorkerLaunch == nil || value.WorkerLaunch.Validate() != nil || !value.Policy.OwnsWorkerProcess || value.Policy.KillOnHardIdle || !inside(value.WorkerLaunch.Launch.RuntimeRoot, value.SessionPath) || overlaps(value.WorkerLaunch.FixtureRoot, value.Identity.SDKRoot) || overlaps(value.WorkerLaunch.Launch.RuntimeRoot, value.Identity.SDKRoot)) {
		return errors.New("worker runtime profile is invalid")
	}
	if value.Role == effect.CoderStart && (value.WorkerLaunch != nil || value.Coder == nil || value.Coder.Validate() != nil || !filepath.IsAbs(value.CoderWorkspace) || value.CoderLaunch == nil || value.CoderLaunch.Validate() != nil || !value.Policy.OwnsWorkerProcess || value.Policy.KillOnHardIdle || !inside(value.CoderLaunch.RuntimeRoot, value.SessionPath) || overlaps(value.CoderWorkspace, value.CoderLaunch.RuntimeRoot) || overlaps(value.CoderWorkspace, value.Identity.SDKRoot) || overlaps(value.CoderLaunch.RuntimeRoot, value.Identity.SDKRoot)) {
		return errors.New("coder runtime profile is invalid")
	}
	if value.ChildSessionDir != "" && !filepath.IsAbs(value.ChildSessionDir) {
		return errors.New("Pi child session directory is invalid")
	}
	return nil
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
	profile, err := h.profile(binding, intent.RunID, ref)
	if err != nil || intent.Kind != profile.Role {
		return nil, errors.New("Pi start profile differs from intent")
	}
	if _, err := piadapter.DiscoverIdentity(profile.Identity); err != nil {
		return nil, errors.New("Pi runtime identity differs from Host binding")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	if intent.Kind == effect.CoderStart {
		profileReceipt, err := op.CoderProfile(intent)
		if err != nil || !reflect.DeepEqual(profileReceipt, *profile.Coder) {
			return nil, errors.New("coder profile differs from Host binding")
		}
		if _, err := coder.Open(binding.store(), profileReceipt.CandidateWorkspace, profileReceipt.SourceSnapshot, profile.CoderWorkspace); err != nil {
			return nil, errors.New("coder workspace differs from Host capability")
		}
		return startPiCoder(binding, op, intent, profile, profileReceipt)
	}
	return startPiWorker(binding, op, intent, profile)
}

func (h *PiRuntimeHost) Poll(binding Binding, runID, ref string) (any, error) {
	profile, err := h.profile(binding, runID, ref)
	if err != nil {
		return nil, err
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, runID)
	if err != nil {
		return nil, err
	}
	return piadapter.Poll(op, profile.SessionPath)
}

func (h *PiRuntimeHost) Checkpoint(binding Binding, intent effect.Intent, ref string) (any, error) {
	profile, err := h.profile(binding, intent.RunID, ref)
	if err != nil || intent.Kind != effect.Checkpoint {
		return nil, errors.New("Pi checkpoint profile differs from intent")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	return piadapter.CheckpointEffect(op, intent, piadapter.CheckpointEffectSpec{SDKRoot: profile.Identity.SDKRoot, ContextFilterPath: profile.Identity.ContextFilterPath, SessionPath: profile.SessionPath})
}

func (h *PiRuntimeHost) Fork(binding Binding, intent effect.Intent, ref string) (any, error) {
	profile, err := h.profile(binding, intent.RunID, ref)
	if err != nil || intent.Kind != effect.Fork || profile.ChildSessionDir == "" {
		return nil, errors.New("Pi fork profile differs from intent")
	}
	op, err := run.Open(binding.Root, binding.ExperimentID, intent.RunID)
	if err != nil {
		return nil, err
	}
	return piadapter.Fork(op, intent, piadapter.ForkSpec{SDKRoot: profile.Identity.SDKRoot, ContextFilterPath: profile.Identity.ContextFilterPath, ParentSession: profile.SessionPath, ChildSessionDir: profile.ChildSessionDir})
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
