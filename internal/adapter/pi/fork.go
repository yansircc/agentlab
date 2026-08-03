package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

const forkAttemptContract = "agentlab.pi-fork-attempt.v1"

type ForkSpec struct {
	SDKRoot         string
	ParentSession   string
	ChildSessionDir string
}

type ForkResult struct {
	Forked  run.SessionForked `json:"forked"`
	Receipt effect.Receipt    `json:"receipt"`
}

type forkAttempt struct {
	Contract        string          `json:"contract"`
	SDKRoot         string          `json:"sdk_root"`
	ParentSession   string          `json:"parent_session"`
	ChildSessionDir string          `json:"child_session_dir"`
	Checkpoint      artifact.Ref    `json:"checkpoint"`
	Identity        AdapterIdentity `json:"identity"`
	EntryID         string          `json:"entry_id"`
	ParentSessionID string          `json:"parent_session_id"`
	PrefixDigest    string          `json:"prefix_digest"`
}

type ForkPayload struct {
	Checkpoint artifact.Ref    `json:"checkpoint"`
	Identity   AdapterIdentity `json:"identity"`
}

func (value ForkPayload) Validate() error {
	if value.Checkpoint.Algorithm != "sha256" || len(value.Checkpoint.Digest) != 64 || value.Checkpoint.Size < 0 || value.Identity.Validate() != nil {
		return errors.New("Pi fork payload is invalid")
	}
	return nil
}

func EncodeForkPayload(value ForkPayload) ([]byte, error) {
	if value.Validate() != nil {
		return nil, errors.New("Pi fork payload is invalid")
	}
	return json.Marshal(value)
}

func Fork(operation *run.Operation, intent effect.Intent, spec ForkSpec) (ForkResult, error) {
	if intent.Kind != effect.Fork || intent.RunID == "" || intent.Validate() != nil || spec.SDKRoot == "" || spec.ParentSession == "" || spec.ChildSessionDir == "" {
		return ForkResult{}, errors.New("Pi fork request is invalid")
	}
	payloadData, err := operation.ReadEffectPayload(intent)
	if err != nil {
		return ForkResult{}, err
	}
	var payload ForkPayload
	if decodeForkJSON(payloadData, &payload) != nil || payload.Validate() != nil {
		return ForkResult{}, errors.New("Pi fork payload is invalid")
	}
	discovered, err := DiscoverIdentity(IdentityConfig{SDKRoot: spec.SDKRoot, AdapterDigest: payload.Identity.AdapterDigest, Provider: payload.Identity.Provider, Model: payload.Identity.Model, ThinkingPolicy: payload.Identity.ThinkingPolicy, CompactionPolicy: payload.Identity.CompactionPolicy})
	if err != nil || !reflect.DeepEqual(discovered, payload.Identity) {
		return ForkResult{}, errors.New("Pi fork adapter identity differs from intent")
	}
	if receipt, exists, err := operation.EffectReceipt(intent.ID); err != nil {
		return ForkResult{}, err
	} else if exists {
		if receipt.Kind != effect.Fork {
			return ForkResult{}, errors.New("Pi fork receipt kind changed")
		}
		return ForkResult{Receipt: receipt}, nil
	}
	checkpoint, prefix, session, opaque, err := operation.RuntimeCheckpointData(payload.Checkpoint)
	if err != nil || checkpoint.Adapter != adapterName {
		return ForkResult{}, errors.New("Pi fork checkpoint is invalid")
	}
	state, err := validateForkParent(spec, checkpoint, session, opaque, prefix, payload.Identity)
	if err != nil {
		return ForkResult{}, err
	}
	attempt, err := newForkAttempt(spec, state, checkpoint, payload.Checkpoint, payload.Identity)
	if err != nil {
		return ForkResult{}, err
	}
	encoded, err := json.Marshal(attempt)
	if err != nil {
		return ForkResult{}, err
	}
	created, err := operation.BeginEffectAttempt(intent, encoded)
	if err != nil {
		return ForkResult{}, err
	}
	childPath := ""
	if created {
		if err := requireEmptyDirectory(spec.ChildSessionDir); err != nil {
			return ForkResult{}, err
		}
		childPath, err = executeForkBridge(attempt)
		if err != nil {
			return ForkResult{}, err
		}
	} else {
		childPath, err = reconcileFork(attempt)
		if err != nil {
			return ForkResult{}, err
		}
	}
	child, childPrefix, err := validateForkChild(childPath, attempt)
	if err != nil || !bytes.Equal(childPrefix, prefix) {
		return ForkResult{}, errors.New("Pi fork child prefix differs from checkpoint")
	}
	identity, err := json.Marshal(payload.Identity)
	if err != nil {
		return ForkResult{}, err
	}
	childSession, err := json.Marshal(sessionReceipt{Contract: checkpointSessionContract, RuntimeLocator: child.RuntimeLocator, Identity: payload.Identity})
	if err != nil {
		return ForkResult{}, err
	}
	forked, err := operation.RecordSessionForked(run.SessionForkSpec{ExpectedCheckpoint: payload.Checkpoint, ChildSession: childSession, ObservedPrefix: childPrefix, AdapterIdentity: identity})
	if err != nil {
		return ForkResult{}, err
	}
	evidence, err := json.Marshal(forked)
	if err != nil {
		return ForkResult{}, err
	}
	receipt, err := operation.SettleEffect(intent, evidence)
	return ForkResult{Forked: forked, Receipt: receipt}, err
}

func newForkAttempt(spec ForkSpec, state checkpointState, checkpoint run.RuntimeCheckpoint, checkpointRef artifact.Ref, identity AdapterIdentity) (forkAttempt, error) {
	if state.SessionID == "" || state.EntryID == "" || state.PrefixDigest != checkpoint.PrefixDigest {
		return forkAttempt{}, errors.New("Pi checkpoint state is invalid")
	}
	return forkAttempt{Contract: forkAttemptContract, SDKRoot: spec.SDKRoot, ParentSession: spec.ParentSession, ChildSessionDir: spec.ChildSessionDir, Checkpoint: checkpointRef, Identity: identity, EntryID: state.EntryID, ParentSessionID: state.SessionID, PrefixDigest: state.PrefixDigest}, nil
}

func requireEmptyDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		return errors.New("Pi child session directory is not empty")
	}
	return nil
}
