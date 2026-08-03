package run

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/transaction"
)

// SessionForked is the generic kernel receipt for an adapter-created child.
// Its fields are artifact identities; adapter-private paths never enter it.
type SessionForked struct {
	ExpectedCheckpoint artifact.Ref `json:"expected_checkpoint"`
	ParentSession      artifact.Ref `json:"parent_session"`
	ChildSession       artifact.Ref `json:"child_session"`
	ObservedPrefix     artifact.Ref `json:"observed_prefix"`
	AdapterIdentity    artifact.Ref `json:"adapter_identity"`
}

type SessionForkSpec struct {
	ExpectedCheckpoint artifact.Ref
	ChildSession       []byte
	ObservedPrefix     []byte
	AdapterIdentity    []byte
}

func (value SessionForked) Validate() error {
	for _, ref := range []artifact.Ref{value.ExpectedCheckpoint, value.ParentSession, value.ChildSession, value.ObservedPrefix, value.AdapterIdentity} {
		if !validRef(ref) {
			return errors.New("session fork receipt is invalid")
		}
	}
	return nil
}

func (o *Operation) RecordSessionForked(spec SessionForkSpec) (SessionForked, error) {
	if !validRef(spec.ExpectedCheckpoint) || len(spec.ChildSession) == 0 || len(spec.ObservedPrefix) == 0 || len(spec.AdapterIdentity) == 0 {
		return SessionForked{}, errors.New("session fork receipt is invalid")
	}
	checkpoint, prefix, err := o.LoadRuntimeCheckpoint(spec.ExpectedCheckpoint)
	if err != nil {
		return SessionForked{}, err
	}
	child, err := o.artifacts.Put(spec.ChildSession)
	if err != nil {
		return SessionForked{}, err
	}
	observed, err := o.artifacts.Put(spec.ObservedPrefix)
	if err != nil {
		return SessionForked{}, err
	}
	identity, err := o.artifacts.Put(spec.AdapterIdentity)
	if err != nil {
		return SessionForked{}, err
	}
	value := SessionForked{ExpectedCheckpoint: spec.ExpectedCheckpoint, ParentSession: checkpoint.Session, ChildSession: child, ObservedPrefix: observed, AdapterIdentity: identity}
	if prefix != value.ObservedPrefix {
		return SessionForked{}, errors.New("session fork receipt does not match checkpoint")
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return SessionForked{}, err
	}
	defer lease.Release()
	state, err := o.currentState()
	if err != nil {
		return SessionForked{}, err
	}
	if state.started == nil || state.terminalSeen {
		return SessionForked{}, errors.New("run cannot record session fork")
	}
	if existing, exists := state.sessionForks[value.ChildSession]; exists {
		if existing != value {
			return SessionForked{}, errors.New("child session has a different fork receipt")
		}
		return existing, nil
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventSessionForked, value); err != nil {
		return SessionForked{}, err
	}
	return value, nil
}
