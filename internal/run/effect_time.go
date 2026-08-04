package run

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
)

// RuntimeCheckpointTime returns when the checkpoint receipt became durable.
// The time is derived from the run ledger; it is never a mutable checkpoint
// field that a caller can backdate.
func (o *Operation) RuntimeCheckpointTime(ref artifact.Ref) (time.Time, error) {
	if _, _, err := o.LoadRuntimeCheckpoint(ref); err != nil {
		return time.Time{}, err
	}
	var result time.Time
	err := o.ledger.Visit(func(record ledger.Record) error {
		if record.Kind != eventRuntimeCheckpoint {
			return nil
		}
		var value runtimeCheckpointRecorded
		if json.Unmarshal(record.Data, &value) != nil {
			return errors.New("runtime checkpoint ledger record is invalid")
		}
		if value.Checkpoint == ref {
			if !result.IsZero() {
				return errors.New("runtime checkpoint has duplicate ledger records")
			}
			result = record.At
		}
		return nil
	})
	if err != nil || result.IsZero() {
		if err == nil {
			err = errors.New("runtime checkpoint has no ledger record")
		}
		return time.Time{}, err
	}
	return result, nil
}

// SessionForkedTime returns the ledger time of the one durable generic child
// session receipt. It makes a retroactive Supervisor decision observable.
func (o *Operation) SessionForkedTime(child artifact.Ref) (time.Time, error) {
	if !child.Valid() {
		return time.Time{}, errors.New("child session reference is invalid")
	}
	state, err := o.currentState()
	if err != nil {
		return time.Time{}, err
	}
	if _, exists := state.sessionForks[child]; !exists {
		return time.Time{}, errors.New("child session fork receipt is unavailable")
	}
	var result time.Time
	err = o.ledger.Visit(func(record ledger.Record) error {
		if record.Kind != eventSessionForked {
			return nil
		}
		var value SessionForked
		if json.Unmarshal(record.Data, &value) != nil || value.Validate() != nil {
			return errors.New("session fork ledger record is invalid")
		}
		if value.ChildSession == child {
			if !result.IsZero() {
				return errors.New("child session has duplicate fork receipts")
			}
			result = record.At
		}
		return nil
	})
	if err != nil || result.IsZero() {
		if err == nil {
			err = errors.New("child session has no fork ledger record")
		}
		return time.Time{}, err
	}
	return result, nil
}
