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
	if err != nil || request.ID == "" || (profile.Role == effect.WorkerStart && request.Handoff != nil) {
		return effect.Intent{}, errors.New("Pi start request is invalid")
	}
	if profile.Role == effect.CoderStart && (request.Handoff == nil || *request.Handoff != profile.Coder.Handoff) {
		return effect.Intent{}, errors.New("coder handoff differs from Host binding")
	}
	payload, err := run.EncodeStartPayload(profile.Role, run.StartPayload{Coder: profile.Coder})
	if err != nil {
		return effect.Intent{}, err
	}
	ref, err := binding.store().Put(payload)
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: profile.Role, Payload: ref}, nil
}

func (h *PiRuntimeHost) CheckpointIntent(binding Binding, request CheckpointRequest) (effect.Intent, error) {
	profile, err := h.profile(binding, request.RunID, request.RuntimeRef)
	if err != nil || request.ID == "" || request.EntryLocator == "" {
		return effect.Intent{}, errors.New("Pi checkpoint request is invalid")
	}
	identity, err := piadapter.DiscoverIdentity(profile.Identity)
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
	profile, err := h.profile(binding, request.RunID, request.RuntimeRef)
	if err != nil || request.ID == "" || !validArtifact(request.Checkpoint) {
		return effect.Intent{}, errors.New("Pi fork request is invalid")
	}
	if _, err := binding.store().Read(request.Checkpoint); err != nil {
		return effect.Intent{}, err
	}
	identity, err := piadapter.DiscoverIdentity(profile.Identity)
	if err != nil {
		return effect.Intent{}, err
	}
	payload, err := piadapter.EncodeForkPayload(piadapter.ForkPayload{Checkpoint: request.Checkpoint, Identity: identity})
	if err != nil {
		return effect.Intent{}, err
	}
	ref, err := binding.store().Put(payload)
	if err != nil {
		return effect.Intent{}, err
	}
	return effect.Intent{ID: request.ID, RunID: request.RunID, Kind: effect.Fork, Payload: ref}, nil
}

func validArtifact(ref artifact.Ref) bool {
	return ref.Algorithm == "sha256" && len(ref.Digest) == 64 && ref.Size >= 0
}
