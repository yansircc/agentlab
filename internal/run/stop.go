package run

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/transaction"
)

type StopResult struct {
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	Reason   string    `json:"reason"`
	Admitted bool      `json:"admitted"`
}

func (o *Operation) RequestStop(reason string) (StopResult, error) {
	result, identity, err := o.admitStop(reason)
	if err != nil {
		return StopResult{}, err
	}
	if identity != nil {
		if err := o.stopManagedProcess(*identity); err != nil {
			return StopResult{}, err
		}
	}
	return result, nil
}

// admitStop writes the sole durable stop transition but never terminates the
// process. Effect callers settle its receipt before performing that external
// termination, so a concurrent Wait cannot terminalize the run first.
func (o *Operation) admitStop(reason string) (StopResult, *processidentity.Identity, error) {
	if reason == "" {
		return StopResult{}, nil, errors.New("stop reason is required")
	}
	state, err := o.currentState()
	if err != nil {
		return StopResult{}, nil, err
	}
	if state.started == nil || state.terminalSeen {
		return StopResult{}, nil, errors.New("run is not stoppable")
	}
	request, err := o.readStopRequest()
	if err != nil {
		return StopResult{}, nil, err
	}
	if request == nil {
		created, err := newStopEvent(reason)
		if err != nil {
			return StopResult{}, nil, err
		}
		data, _ := json.Marshal(created)
		if err := transaction.WriteOnce(filepath.Join(o.dir, "stop.request"), data, 0o600); err != nil {
			return StopResult{}, nil, err
		}
		request = &created
	}
	result := StopResult{ID: request.ID, At: request.At, Reason: request.Reason, Admitted: state.stopRequested}
	if state.started.Process.Kind == processManaged {
		if !state.stopRequested {
			lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
			if errors.Is(err, transaction.ErrLeaseHeld) {
				return result, nil, nil
			}
			if err != nil {
				return StopResult{}, nil, err
			}
			defer lease.Release()
			if err := o.admitPendingStop(); err != nil {
				return StopResult{}, nil, err
			}
			result.Admitted = true
		}
		if state.started.Process.Identity == nil {
			return StopResult{}, nil, errors.New("managed process identity is absent")
		}
		identity := *state.started.Process.Identity
		return result, &identity, nil
	}
	if state.started.Process.Kind != processAttached || state.stopRequested {
		return result, nil, nil
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if errors.Is(err, transaction.ErrLeaseHeld) {
		return result, nil, nil
	}
	if err != nil {
		return StopResult{}, nil, err
	}
	defer lease.Release()
	if err := o.admitPendingStop(); err != nil {
		return StopResult{}, nil, err
	}
	result.Admitted = true
	return result, nil, nil
}

func (o *Operation) stopManagedProcess(identity processidentity.Identity) error {
	switch o.attemptProber.Observe(identity) {
	case processidentity.Dead, processidentity.Mismatch:
		return nil
	case processidentity.Matches:
		return o.terminateIdentity(identity)
	default:
		return errors.New("managed process identity is unverifiable")
	}
}

func (o *Operation) admitPendingStop() error {
	state, err := o.currentState()
	if err != nil || state.stopRequested || state.terminalSeen {
		return err
	}
	request, err := o.readStopRequest()
	if err != nil || request == nil {
		return err
	}
	_, err = o.appendEvent(time.Now().UTC(), eventStopRequested, *request)
	return err
}

func (o *Operation) admitOwnedStop(reason string) (*stopEvent, error) {
	if reason != "" {
		if _, err := o.RequestStop(reason); err != nil {
			return nil, err
		}
	}
	state, err := o.currentState()
	if err != nil || state.stopRequested {
		return nil, err
	}
	request, err := o.readStopRequest()
	if err != nil || request == nil {
		return nil, err
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventStopRequested, *request); err != nil {
		return nil, err
	}
	return request, nil
}

func (o *Operation) readStopRequest() (*stopEvent, error) {
	data, err := os.ReadFile(filepath.Join(o.dir, "stop.request"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value stopEvent
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid stop request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || value.ID == "" || value.At.IsZero() || value.Reason == "" {
		return nil, errors.New("invalid stop request")
	}
	return &value, nil
}

func (o *Operation) currentState() (replayState, error) {
	records, err := o.ledger.Replay()
	if err != nil {
		return replayState{}, err
	}
	return replayRun(records)
}

func newStopEvent(reason string) (stopEvent, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return stopEvent{}, err
	}
	return stopEvent{ID: hex.EncodeToString(bytes), At: time.Now().UTC(), Reason: reason}, nil
}
