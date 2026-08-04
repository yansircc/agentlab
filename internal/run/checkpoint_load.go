package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
)

func (o *Operation) LoadRuntimeCheckpoint(ref artifact.Ref) (RuntimeCheckpoint, artifact.Ref, error) {
	if !validRef(ref) {
		return RuntimeCheckpoint{}, artifact.Ref{}, errors.New("runtime checkpoint reference is invalid")
	}
	state, err := o.currentState()
	if err != nil {
		return RuntimeCheckpoint{}, artifact.Ref{}, err
	}
	record, ok := state.runtimeCheckpoints[ref]
	if !ok {
		return RuntimeCheckpoint{}, artifact.Ref{}, ErrCheckpointNotFound
	}
	data, err := o.artifacts.Read(ref)
	if err != nil {
		return RuntimeCheckpoint{}, artifact.Ref{}, err
	}
	var value RuntimeCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || value.Validate() != nil {
		return RuntimeCheckpoint{}, artifact.Ref{}, errors.New("runtime checkpoint artifact is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RuntimeCheckpoint{}, artifact.Ref{}, errors.New("runtime checkpoint artifact has trailing input")
	}
	if value.PrefixDigest != record.PublicPrefix.Digest || value.Intent != record.Intent || value.Intent.RunID != o.runID {
		return RuntimeCheckpoint{}, artifact.Ref{}, errors.New("runtime checkpoint prefix digest differs from recorded prefix")
	}
	if _, err := o.artifacts.Read(record.PublicPrefix); err != nil {
		return RuntimeCheckpoint{}, artifact.Ref{}, err
	}
	return value, record.PublicPrefix, nil
}

func (o *Operation) RuntimeCheckpointData(ref artifact.Ref) (RuntimeCheckpoint, []byte, []byte, []byte, error) {
	value, prefixRef, err := o.LoadRuntimeCheckpoint(ref)
	if err != nil {
		return RuntimeCheckpoint{}, nil, nil, nil, err
	}
	prefix, err := o.artifacts.Read(prefixRef)
	if err != nil {
		return RuntimeCheckpoint{}, nil, nil, nil, err
	}
	session, err := o.artifacts.Read(value.Session)
	if err != nil {
		return RuntimeCheckpoint{}, nil, nil, nil, err
	}
	opaque, err := o.artifacts.Read(value.OpaqueState)
	if err != nil {
		return RuntimeCheckpoint{}, nil, nil, nil, err
	}
	return value, prefix, session, opaque, nil
}

// VerifyCheckpointEffect proves that one settled checkpoint intent created
// exactly one durable checkpoint receipt. Adapter implementations remain
// responsible for validating their opaque runtime state before recording it.
func (o *Operation) VerifyCheckpointEffect(intent effect.Intent) (RuntimeCheckpointRecord, error) {
	if intent.RunID != o.runID || intent.Kind != effect.Checkpoint || intent.Validate() != nil {
		return RuntimeCheckpointRecord{}, errors.New("checkpoint effect intent is invalid")
	}
	if _, _, err := o.verifyObservedReceipt(intent); err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	state, err := o.currentState()
	if err != nil {
		return RuntimeCheckpointRecord{}, err
	}
	var result RuntimeCheckpointRecord
	for ref, recorded := range state.runtimeCheckpoints {
		if recorded.Intent != intent {
			continue
		}
		if result.Checkpoint.Valid() {
			return RuntimeCheckpointRecord{}, errors.New("checkpoint effect has multiple receipts")
		}
		checkpoint, prefix, err := o.LoadRuntimeCheckpoint(ref)
		if err != nil || checkpoint.Intent != intent || prefix != recorded.PublicPrefix {
			return RuntimeCheckpointRecord{}, errors.New("checkpoint effect differs from run ledger")
		}
		result = RuntimeCheckpointRecord{Checkpoint: ref, PublicPrefix: prefix, Intent: intent}
	}
	if !result.Checkpoint.Valid() {
		return RuntimeCheckpointRecord{}, errors.New("checkpoint effect has no runtime receipt")
	}
	return result, nil
}
