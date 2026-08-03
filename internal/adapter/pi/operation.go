package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
)

const adapterName = "pi-session-v3"

type PollResult struct {
	SessionID  string `json:"session_id"`
	Offset     int64  `json:"offset"`
	BatchCount int    `json:"batch_count"`
	EventCount int    `json:"event_count"`
	Excluded   int    `json:"excluded_count"`
	Stopped    bool   `json:"stopped"`
}

type BeginResult struct {
	SessionID string `json:"session_id"`
	Offset    int64  `json:"offset"`
}

func Begin(operation *run.Operation, sessionPath string, policy run.StopPolicy, identity *processidentity.Identity) (BeginResult, error) {
	cursor, err := Attach(sessionPath)
	if err != nil {
		return BeginResult{}, err
	}
	encoded, err := encodeCursor(cursor)
	if err != nil {
		return BeginResult{}, err
	}
	if _, err := operation.BeginAttached(run.AttachedSpec{
		Adapter:       adapterName,
		StreamID:      cursor.SessionID,
		InitialCursor: encoded,
		Policy:        policy,
		Identity:      identity,
		Capabilities:  run.RequiredAdapterCapabilities(),
	}); err != nil {
		return BeginResult{}, err
	}
	return BeginResult{SessionID: cursor.SessionID, Offset: cursor.Offset}, nil
}

func Poll(operation *run.Operation, sessionPath string) (result PollResult, resultErr error) {
	writer, state, err := operation.AcquireAdapterWriter(adapterName)
	if err != nil {
		return PollResult{}, err
	}
	defer func() {
		if err := writer.Close(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	if state.Stopped {
		cursor, err := decodeCursor(state.Cursor)
		if err != nil {
			return PollResult{}, err
		}
		return PollResult{SessionID: cursor.SessionID, Offset: cursor.Offset, Stopped: true}, nil
	}
	cursor, err := decodeCursor(state.Cursor)
	if err != nil || cursor.SessionID != state.StreamID {
		return PollResult{}, fmt.Errorf("invalid durable Pi cursor")
	}
	sink := operationSink{writer: writer}
	next, err := ReadNew(sessionPath, cursor, &sink)
	if err != nil {
		return PollResult{}, err
	}
	return PollResult{
		SessionID:  next.SessionID,
		Offset:     next.Offset,
		BatchCount: sink.batches,
		EventCount: sink.events,
		Excluded:   sink.excluded,
	}, nil
}

type operationSink struct {
	writer   *run.AdapterWriter
	batches  int
	events   int
	excluded int
}

func (s *operationSink) Commit(next Cursor, batch Batch) error {
	encoded, err := encodeCursor(next)
	if err != nil {
		return err
	}
	input := run.AdapterBatch{}
	for _, item := range batch.Events {
		input.Events = append(input.Events, run.AdapterEvent{
			Kind: run.EvidenceKind(item.Kind), CorrelationID: item.CorrelationID, Raw: item.Raw,
			Label: item.Label, CompactText: item.CompactText,
		})
	}
	for _, item := range batch.Exclusions {
		input.Exclusions = append(input.Exclusions, run.AdapterExcluded{Category: item.Category, Size: item.Size})
	}
	if err := s.writer.Commit(encoded, input); err != nil {
		return err
	}
	s.batches++
	s.events += len(batch.Events)
	s.excluded += len(batch.Exclusions)
	return nil
}

func encodeCursor(cursor Cursor) ([]byte, error) {
	if cursor.SessionID == "" || cursor.Offset < 0 {
		return nil, ErrInvalidSession
	}
	return json.Marshal(cursor)
}

func decodeCursor(data []byte) (Cursor, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cursor Cursor
	if err := decoder.Decode(&cursor); err != nil {
		return Cursor{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || cursor.SessionID == "" || cursor.Offset < 0 {
		return Cursor{}, ErrInvalidSession
	}
	return cursor, nil
}
