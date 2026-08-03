package run

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/transaction"
)

func (o *Operation) AcquireAdapterWriter(adapter string) (*AdapterWriter, AdapterState, error) {
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return nil, AdapterState{}, err
	}
	if err := o.admitPendingStop(); err != nil {
		_ = lease.Release()
		return nil, AdapterState{}, err
	}
	state, err := o.loadAdapterState(adapter)
	if err != nil {
		_ = lease.Release()
		return nil, AdapterState{}, err
	}
	writer := &AdapterWriter{operation: o, lease: lease, adapter: adapter, streamID: state.StreamID, cursor: append([]byte(nil), state.Cursor...)}
	return writer, state, nil
}

func (o *Operation) AdapterState(adapter string) (AdapterState, error) {
	return o.loadAdapterState(adapter)
}

func (w *AdapterWriter) Commit(nextCursor []byte, input AdapterBatch) error {
	if w.closed {
		return errors.New("adapter writer is closed")
	}
	if len(nextCursor) == 0 || bytes.Equal(nextCursor, w.cursor) {
		return errors.New("adapter cursor must advance")
	}
	if len(input.Events)+len(input.Exclusions) > 1000 {
		return errors.New("adapter batch exceeds evidence bound")
	}
	cursorRef, err := w.operation.artifacts.Put(nextCursor)
	if err != nil {
		return err
	}
	value := adapterBatch{Adapter: w.adapter, StreamID: w.streamID, Cursor: cursorRef}
	for _, item := range input.Events {
		if !item.Kind.valid() || item.Label == "" || len(item.Label) > 128 || len(item.CorrelationID) > 256 || len(item.CompactText) > 4096 {
			return errors.New("adapter event kind and label are required")
		}
		raw, err := w.operation.artifacts.Put(item.Raw)
		if err != nil {
			return err
		}
		value.Admissions = append(value.Admissions, adapterAdmission{Kind: item.Kind, CorrelationID: item.CorrelationID, Raw: raw, Label: item.Label, CompactText: item.CompactText})
	}
	for _, item := range input.Exclusions {
		if item.Category == "" || len(item.Category) > 128 || item.Size < 0 {
			return errors.New("adapter exclusion is invalid")
		}
		value.Exclusions = append(value.Exclusions, adapterExclusion{Category: item.Category, Size: item.Size})
	}
	if _, err := w.operation.appendEvent(time.Now().UTC(), eventAdapterBatch, value); err != nil {
		return err
	}
	w.cursor = append(w.cursor[:0], nextCursor...)
	return nil
}

func (w *AdapterWriter) Close() error {
	if w.closed {
		return errors.New("adapter writer is already closed")
	}
	w.closed = true
	return w.lease.Release()
}

func (o *Operation) loadAdapterState(adapter string) (AdapterState, error) {
	records, err := o.ledger.Replay()
	if err != nil {
		return AdapterState{}, err
	}
	state, err := replayRun(records)
	if err != nil {
		return AdapterState{}, err
	}
	if state.started == nil || state.started.Adapter == nil || state.started.Adapter.Adapter != adapter {
		return AdapterState{}, fmt.Errorf("run is not attached through adapter %q", adapter)
	}
	cursor, err := o.artifacts.Read(state.adapterCursor)
	if err != nil {
		return AdapterState{}, err
	}
	return AdapterState{Adapter: adapter, StreamID: state.started.Adapter.StreamID, Cursor: cursor, Stopped: state.stopRequested}, nil
}
