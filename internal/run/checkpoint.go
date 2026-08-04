package run

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/transaction"
)

const RuntimeCheckpointContract = "agentlab.runtime-checkpoint.v2"

// RuntimeCheckpoint is the generic kernel receipt for an adapter-owned fork point.
type RuntimeCheckpoint struct {
	Contract     string        `json:"contract"`
	Intent       effect.Intent `json:"intent"`
	Adapter      string        `json:"adapter"`
	Session      artifact.Ref  `json:"session"`
	OpaqueState  artifact.Ref  `json:"opaque_state"`
	PrefixDigest string        `json:"prefix_digest"`
}

type RuntimeCheckpointSpec struct {
	Adapter      string
	Session      []byte
	OpaqueState  []byte
	PublicPrefix []byte
}

type RuntimeCheckpointRecord struct {
	Checkpoint   artifact.Ref
	PublicPrefix artifact.Ref
	Intent       effect.Intent
}

func (c RuntimeCheckpoint) Validate() error {
	if c.Contract != RuntimeCheckpointContract || c.Intent.Validate() != nil || c.Intent.Kind != effect.Checkpoint || c.Adapter == "" || len(c.Adapter) > 128 || !validRef(c.Session) || !validRef(c.OpaqueState) || len(c.PrefixDigest) != 64 {
		return errors.New("runtime checkpoint is invalid")
	}
	if _, err := hex.DecodeString(c.PrefixDigest); err != nil {
		return errors.New("runtime checkpoint prefix digest is invalid")
	}
	return nil
}

func (s RuntimeCheckpointSpec) Validate() error {
	if s.Adapter == "" || len(s.Adapter) > 128 || len(s.Session) == 0 || len(s.OpaqueState) == 0 || len(s.PublicPrefix) == 0 {
		return errors.New("runtime checkpoint payload is invalid")
	}
	return nil
}

func (o *Operation) RecordRuntimeCheckpoint(intent effect.Intent, spec RuntimeCheckpointSpec) (result RuntimeCheckpointRecord, resultErr error) {
	if spec.Validate() != nil || intent.Validate() != nil || intent.RunID != o.runID || intent.Kind != effect.Checkpoint {
		return RuntimeCheckpointRecord{}, errors.New("runtime checkpoint effect is invalid")
	}
	if _, err := o.ReadEffectPayload(intent); err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	defer func() {
		if err := lease.Release(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	records, err := o.ledger.Replay()
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	state, err := replayRun(records)
	if err != nil || state.started == nil || state.terminalSeen || state.started.Adapter == nil || state.started.Adapter.Adapter != spec.Adapter {
		return RuntimeCheckpointRecord{}, errors.New("runtime checkpoint is not admissible for this run")
	}
	prefix, err := o.artifacts.Put(spec.PublicPrefix)
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	session, err := o.artifacts.Put(spec.Session)
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	opaque, err := o.artifacts.Put(spec.OpaqueState)
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	value := RuntimeCheckpoint{Contract: RuntimeCheckpointContract, Intent: intent, Adapter: spec.Adapter, Session: session, OpaqueState: opaque, PrefixDigest: prefix.Digest}
	data, err := json.Marshal(value)
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	ref, err := o.artifacts.Put(data)
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	if existing, ok := state.runtimeCheckpoints[ref]; ok {
		if existing.PublicPrefix != prefix || existing.Intent != intent {
			return RuntimeCheckpointRecord{}, errors.New("runtime checkpoint prefix identity changed")
		}
		return RuntimeCheckpointRecord{Checkpoint: ref, PublicPrefix: prefix, Intent: intent}, nil
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventRuntimeCheckpoint, runtimeCheckpointRecorded{Checkpoint: ref, PublicPrefix: prefix, Intent: intent}); err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	return RuntimeCheckpointRecord{Checkpoint: ref, PublicPrefix: prefix, Intent: intent}, nil
}

func (o *Operation) HasRuntimeCheckpoint(ref artifact.Ref) (bool, error) {
	_, err := o.RuntimeCheckpointPublicPrefix(ref)
	if errors.Is(err, ErrCheckpointNotFound) {
		return false, nil
	}
	return err == nil, err
}

var ErrCheckpointNotFound = errors.New("runtime checkpoint does not belong to run")

func (o *Operation) RuntimeCheckpointPublicPrefix(ref artifact.Ref) (artifact.Ref, error) {
	_, prefix, err := o.LoadRuntimeCheckpoint(ref)
	return prefix, err
}
