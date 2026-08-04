package tool

import (
	"errors"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func (h *PiRuntimeHost) StartIntent(binding Binding, request StartRequest) (effect.Intent, error) {
	profile, err := h.profile(binding, request.RunID, request.RuntimeRef)
	worker := false
	if err != nil {
		if _, workerErr := h.activeWorkerProfile(binding, request.RunID, request.RuntimeRef); workerErr != nil {
			return effect.Intent{}, errors.New("Pi start request is invalid")
		}
		worker = true
	}
	if request.ID == "" || (worker && request.Handoff != nil) || (!worker && profile.Role == effect.WorkerStart && request.Handoff != nil) {
		return effect.Intent{}, errors.New("Pi start request is invalid")
	}
	payload := run.StartPayload{}
	if !worker && profile.Role == effect.CoderStart {
		if request.Handoff == nil {
			return effect.Intent{}, errors.New("coder handoff is required")
		}
		experiment, err := binding.experiment()
		if err != nil {
			return effect.Intent{}, err
		}
		if _, err := experiment.Handoff(*request.Handoff); err != nil {
			return effect.Intent{}, errors.New("coder handoff is outside experiment")
		}
		payload.Coder = &run.CoderProfile{Handoff: *request.Handoff, SourceSnapshot: profile.CoderSourceSnapshot, CandidateWorkspace: profile.CoderWorkspaceReceipt, CapabilityProfile: profile.CoderCapabilityProfile}
	}
	role := profile.Role
	if worker {
		role = effect.WorkerStart
	}
	payloadBytes, err := run.EncodeStartPayload(role, payload)
	if err != nil {
		return effect.Intent{}, err
	}
	ref, err := binding.store().Put(payloadBytes)
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: role, Payload: ref}, nil
}

func (h *PiRuntimeHost) CheckpointIntent(binding Binding, request CheckpointRequest) (effect.Intent, error) {
	profile, err := h.activeWorkerProfile(binding, request.RunID, request.RuntimeRef)
	if err != nil || request.ID == "" || request.EntryLocator == "" {
		return effect.Intent{}, errors.New("Pi checkpoint request is invalid")
	}
	identity, err := piadapter.VerifyRuntimeIdentity(profile.Identity)
	if err != nil {
		return effect.Intent{}, err
	}
	payload, err := piadapter.EncodeCheckpointPayload(piadapter.CheckpointPayload{EntryLocator: request.EntryLocator, Identity: identity})
	if err != nil {
		return effect.Intent{}, err
	}
	ref, err := binding.store().Put(payload)
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: effect.Checkpoint, Payload: ref}, nil
}

func (h *PiRuntimeHost) ForkIntent(binding Binding, request ForkRequest) (effect.Intent, error) {
	profile, err := h.activeWorkerProfile(binding, request.RunID, request.RuntimeRef)
	if err != nil || request.ID == "" || !validArtifact(request.Checkpoint) || request.ChildRun == "" {
		return effect.Intent{}, errors.New("Pi fork request is invalid")
	}
	template, err := h.preparedWorkerForRun(binding, request.ChildRun)
	if err != nil {
		return effect.Intent{}, errors.New("Pi fork child runtime is absent")
	}
	manifest, _, err := preparedWorkerManifest(binding, template)
	if err != nil {
		return effect.Intent{}, err
	}
	origin, ok := manifest.Origin.Splice()
	if !ok || origin.ParentRun != request.RunID || origin.RuntimeCheckpoint != request.Checkpoint {
		return effect.Intent{}, errors.New("Pi fork child origin is invalid")
	}
	if template.Forked != nil && (template.Forked.ParentRun != request.RunID || template.Forked.ForkReceipt.IntentID != request.ID || template.Forked.Forked.ExpectedCheckpoint != request.Checkpoint) {
		return effect.Intent{}, errors.New("Pi fork child is already bound to another receipt")
	}
	if _, err := binding.store().Read(request.Checkpoint); err != nil {
		return effect.Intent{}, err
	}
	identity, err := piadapter.VerifyRuntimeIdentity(profile.Identity)
	if err != nil {
		return effect.Intent{}, err
	}
	payload, err := piadapter.EncodeForkPayload(piadapter.ForkPayload{Checkpoint: request.Checkpoint, Identity: identity, ChildRun: request.ChildRun})
	if err != nil {
		return effect.Intent{}, err
	}
	ref, err := binding.store().Put(payload)
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: effect.Fork, Payload: ref}, nil
}

func validArtifact(ref artifact.Ref) bool { return ref.Valid() }
