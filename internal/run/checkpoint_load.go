package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/yansircc/agentlab/internal/artifact"
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
	if value.PrefixDigest != record.PublicPrefix.Digest {
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
