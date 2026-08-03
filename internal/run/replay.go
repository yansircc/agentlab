package run

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/ledger"
)

type replayState struct {
	started            *processStarted
	exit               *processExited
	terminalAccepted   bool
	terminalRejected   bool
	terminalSeen       bool
	streamCorrupt      bool
	closedStreams      map[string]bool
	lastEvidenceAt     *time.Time
	startedAt          *time.Time
	lastEventAt        *time.Time
	semanticProgress   SemanticProgress
	firstEventTimedOut bool
	adapterCursor      artifact.Ref
	stopRequested      bool
	runtimeCheckpoints map[artifact.Ref]runtimeCheckpointRecorded
	effectReceipts     map[string]effect.Receipt
	sessionForks       map[artifact.Ref]SessionForked
}

func replayRun(records []ledger.Record) (replayState, error) {
	state := initialReplayState()
	for _, record := range records {
		if err := state.apply(record); err != nil {
			return replayState{}, err
		}
	}
	return state, nil
}

func initialReplayState() replayState {
	return replayState{closedStreams: map[string]bool{}, runtimeCheckpoints: map[artifact.Ref]runtimeCheckpointRecorded{}, effectReceipts: map[string]effect.Receipt{}, sessionForks: map[artifact.Ref]SessionForked{}, semanticProgress: ProgressUnknown}
}

func (s *replayState) apply(record ledger.Record) error {
	if s.terminalSeen {
		return fmt.Errorf("event after terminal fact at sequence %d", record.Sequence)
	}
	if record.Kind != eventProcessStarted && s.started == nil {
		return fmt.Errorf("run event precedes process_started at sequence %d", record.Sequence)
	}
	at := record.At
	s.lastEventAt = &at
	switch record.Kind {
	case eventProcessStarted:
		return s.start(record)
	case eventEvidence:
		return s.evidence(record)
	case eventAdapterBatch:
		return s.adapter(record)
	case eventProgressObserved, eventNoProgress:
		return s.progress(record)
	case eventFirstTimeout:
		if s.exit != nil {
			return invalid(record, "first_event_timeout after exit")
		}
		s.firstEventTimedOut = true
	case eventSoftIdle, eventHardIdle:
		if s.exit != nil {
			return invalid(record, "idle fact after exit")
		}
	case eventStreamClosed, eventStreamCorrupt:
		return s.stream(record)
	case eventProcessExited:
		return s.exited(record)
	case eventTerminalAccepted, eventTerminalRejected:
		return s.terminal(record)
	case eventStopRequested:
		var value stopEvent
		if s.exit != nil || s.stopRequested || json.Unmarshal(record.Data, &value) != nil || value.ID == "" || value.At.IsZero() || value.Reason == "" {
			return invalid(record, "stop_requested after exit")
		}
		s.stopRequested = true
	case eventRuntimeCheckpoint:
		return s.checkpoint(record)
	case eventEffectReceipt:
		return s.effectReceipt(record)
	case eventSessionForked:
		return s.sessionForked(record)
	default:
		return fmt.Errorf("unknown run event kind %q at sequence %d", record.Kind, record.Sequence)
	}
	return nil
}

func (s *replayState) checkpoint(record ledger.Record) error {
	var value runtimeCheckpointRecorded
	if s.exit != nil || s.stopRequested || s.started == nil || s.started.Adapter == nil || json.Unmarshal(record.Data, &value) != nil || !validRef(value.Checkpoint) || !validRef(value.PublicPrefix) {
		return invalid(record, "invalid runtime checkpoint")
	}
	if _, exists := s.runtimeCheckpoints[value.Checkpoint]; exists {
		return invalid(record, "invalid runtime checkpoint")
	}
	s.runtimeCheckpoints[value.Checkpoint] = value
	return nil
}

func (s *replayState) start(record ledger.Record) error {
	var value processStarted
	if s.started != nil || record.Sequence != 1 || json.Unmarshal(record.Data, &value) != nil || value.Policy.Validate() != nil || !validProcessStarted(value) {
		return invalid(record, "invalid process_started")
	}
	s.started = &value
	if value.Adapter != nil {
		s.adapterCursor = value.Adapter.Cursor
	}
	at := record.At
	s.startedAt = &at
	return nil
}

func validProcessStarted(value processStarted) bool {
	if !validRef(value.Manifest) {
		return false
	}
	identityValid := value.Process.Identity == nil || (value.Process.Identity.PID > 0 && value.Process.Identity.StartToken != "" && value.Process.Identity.CommandHash != "")
	switch value.Process.Kind {
	case processOwned:
		return value.AttemptID != "" && identityValid && value.Process.Identity != nil && value.Policy.OwnsWorkerProcess && value.Adapter == nil
	case processAttached:
		return value.AttemptID == "" && identityValid && !value.Policy.OwnsWorkerProcess && !value.Policy.KillOnHardIdle && value.Adapter != nil && value.Adapter.Adapter != "" && value.Adapter.StreamID != "" && validRef(value.Adapter.Cursor) && value.Adapter.Capabilities == RequiredAdapterCapabilities()
	default:
		return false
	}
}

func (s *replayState) evidence(record ledger.Record) error {
	var value evidence
	if json.Unmarshal(record.Data, &value) != nil || (value.Stream != "stdout" && value.Stream != "stderr") || s.closedStreams[value.Stream] || s.exit != nil || value.Label == "" || !validRef(value.Raw) {
		return invalid(record, "invalid evidence")
	}
	at := record.At
	s.lastEvidenceAt = &at
	return nil
}

func (s *replayState) adapter(record ledger.Record) error {
	var value adapterBatch
	if json.Unmarshal(record.Data, &value) != nil || s.exit != nil || s.started.Adapter == nil || value.Adapter != s.started.Adapter.Adapter || value.StreamID != s.started.Adapter.StreamID || !validRef(value.Cursor) {
		return invalid(record, "invalid adapter_batch")
	}
	for _, item := range value.Admissions {
		if !item.Kind.valid() || item.Label == "" || !validRef(item.Raw) {
			return invalid(record, "invalid adapter admission")
		}
	}
	for _, item := range value.Exclusions {
		if item.Category == "" || item.Size < 0 {
			return invalid(record, "invalid adapter exclusion")
		}
	}
	if len(value.Admissions)+len(value.Exclusions) > 0 {
		at := record.At
		s.lastEvidenceAt = &at
	}
	s.adapterCursor = value.Cursor
	return nil
}

func (s *replayState) progress(record ledger.Record) error {
	var value progressFact
	if json.Unmarshal(record.Data, &value) != nil || value.Detector == "" || s.exit != nil {
		return invalid(record, "invalid progress fact")
	}
	if record.Kind == eventProgressObserved {
		s.semanticProgress = ProgressObserved
	} else {
		s.semanticProgress = NoProgressEvidence
	}
	return nil
}

func validRef(ref artifact.Ref) bool {
	return ref.Valid()
}

func invalid(record ledger.Record, reason string) error {
	return fmt.Errorf("%s at sequence %d", reason, record.Sequence)
}
