package pi

import (
	"encoding/json"
	"errors"
	"reflect"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const checkpointAttemptContract = "agentlab.pi-checkpoint-attempt.v1"

type CheckpointEffectSpec struct {
	SDKRoot           string
	ContextFilterPath string
	SessionPath       string
}

type CheckpointPayload struct {
	EntryLocator string          `json:"entry_locator"`
	Identity     AdapterIdentity `json:"identity"`
}

type CheckpointEffectResult struct {
	Checkpoint CheckpointResult `json:"checkpoint"`
	Receipt    effect.Receipt   `json:"receipt"`
}

type checkpointAttempt struct {
	Contract    string            `json:"contract"`
	SessionPath string            `json:"session_path"`
	Payload     CheckpointPayload `json:"payload"`
}

func EncodeCheckpointPayload(value CheckpointPayload) ([]byte, error) {
	if value.EntryLocator == "" || len(value.EntryLocator) > 256 || value.Identity.Validate() != nil {
		return nil, errors.New("Pi checkpoint payload is invalid")
	}
	return json.Marshal(value)
}

func CheckpointEffect(operation *run.Operation, intent effect.Intent, spec CheckpointEffectSpec) (CheckpointEffectResult, error) {
	if intent.Kind != effect.Checkpoint || intent.RunID == "" || intent.Validate() != nil || spec.SDKRoot == "" || spec.SessionPath == "" {
		return CheckpointEffectResult{}, errors.New("Pi checkpoint request is invalid")
	}
	payloadData, err := operation.ReadEffectPayload(intent)
	if err != nil {
		return CheckpointEffectResult{}, err
	}
	var payload CheckpointPayload
	if strictjson.Decode(payloadData, &payload) != nil {
		return CheckpointEffectResult{}, errors.New("Pi checkpoint payload is invalid")
	}
	if _, err := EncodeCheckpointPayload(payload); err != nil {
		return CheckpointEffectResult{}, err
	}
	discovered, err := DiscoverIdentity(IdentityConfig{SDKRoot: spec.SDKRoot, ContextFilterPath: spec.ContextFilterPath, AdapterDigest: payload.Identity.AdapterDigest, Provider: payload.Identity.Provider, Model: payload.Identity.Model, ThinkingPolicy: payload.Identity.ThinkingPolicy, CompactionPolicy: payload.Identity.CompactionPolicy})
	if err != nil || !reflect.DeepEqual(discovered, payload.Identity) {
		return CheckpointEffectResult{}, errors.New("Pi checkpoint adapter identity differs from intent")
	}
	attempt, err := json.Marshal(checkpointAttempt{Contract: checkpointAttemptContract, SessionPath: spec.SessionPath, Payload: payload})
	if err != nil {
		return CheckpointEffectResult{}, err
	}
	created, err := operation.BeginEffectAttempt(intent, attempt)
	if err != nil {
		return CheckpointEffectResult{}, err
	}
	if !created {
		return reconcileCheckpointEffect(operation, intent)
	}
	checkpoint, err := Checkpoint(operation, spec.SessionPath, payload.EntryLocator, payload.Identity)
	if err != nil {
		return CheckpointEffectResult{}, err
	}
	evidence, err := json.Marshal(checkpoint)
	if err != nil {
		return CheckpointEffectResult{}, err
	}
	if err := operation.RecordEffectObservation(intent, evidence); err != nil {
		return CheckpointEffectResult{}, err
	}
	receipt, err := operation.SettleEffect(intent, evidence)
	return CheckpointEffectResult{Checkpoint: checkpoint, Receipt: receipt}, err
}

func reconcileCheckpointEffect(operation *run.Operation, intent effect.Intent) (CheckpointEffectResult, error) {
	evidence, exists, err := operation.EffectObservation(intent)
	if err != nil || !exists {
		return CheckpointEffectResult{}, errors.New("Pi checkpoint outcome is unknown; refusing to repeat it")
	}
	var checkpoint CheckpointResult
	if strictjson.Decode(evidence, &checkpoint) != nil {
		return CheckpointEffectResult{}, errors.New("Pi checkpoint observation is invalid")
	}
	prefix, err := operation.RuntimeCheckpointPublicPrefix(checkpoint.Checkpoint.Checkpoint)
	if err != nil || prefix != checkpoint.Checkpoint.PublicPrefix || prefix.Digest != checkpoint.PrefixDigest {
		return CheckpointEffectResult{}, errors.New("Pi checkpoint observation no longer matches run")
	}
	if receipt, exists, err := operation.EffectReceipt(intent.ID); err != nil {
		return CheckpointEffectResult{}, err
	} else if exists {
		return CheckpointEffectResult{Checkpoint: checkpoint, Receipt: receipt}, nil
	}
	receipt, err := operation.SettleEffect(intent, evidence)
	return CheckpointEffectResult{Checkpoint: checkpoint, Receipt: receipt}, err
}
