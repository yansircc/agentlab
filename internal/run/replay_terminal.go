package run

import (
	"encoding/json"

	"github.com/yansircc/agentlab/internal/ledger"
)

func (s *replayState) stream(record ledger.Record) error {
	var value streamFact
	if json.Unmarshal(record.Data, &value) != nil || (value.Stream != "stdout" && value.Stream != "stderr") || s.exit != nil {
		return invalid(record, "invalid stream fact")
	}
	if record.Kind == eventStreamClosed {
		if s.closedStreams[value.Stream] {
			return invalid(record, "duplicate stream_closed")
		}
		s.closedStreams[value.Stream] = true
		return nil
	}
	if value.Error == "" {
		return invalid(record, "stream_corrupt without error")
	}
	s.streamCorrupt = true
	return nil
}

func (s *replayState) exited(record ledger.Record) error {
	var value processExited
	if json.Unmarshal(record.Data, &value) != nil || s.exit != nil || !validExit(s, value) {
		return invalid(record, "invalid process_exited")
	}
	s.exit = &value
	return nil
}

func validExit(state *replayState, value processExited) bool {
	if state.started.Process.Kind == processOwned {
		return state.closedStreams["stdout"] && state.closedStreams["stderr"]
	}
	return state.started.Process.Kind == processManaged
}

func (s *replayState) terminal(record ledger.Record) error {
	if s.exit == nil {
		return invalid(record, "terminal fact before exit")
	}
	if record.Kind == eventTerminalAccepted {
		var value terminalResult
		if json.Unmarshal(record.Data, &value) != nil || value.Outcome != "success" || !validManagedTerminal(s, value) {
			return invalid(record, "invalid terminal_accepted")
		}
		s.terminalAccepted = true
	} else {
		var value terminalRejected
		if json.Unmarshal(record.Data, &value) != nil || value.Reason == "" {
			return invalid(record, "invalid terminal_rejected")
		}
		s.terminalRejected = true
	}
	s.terminalSeen = true
	return nil
}

func validManagedTerminal(state *replayState, value terminalResult) bool {
	switch state.started.Process.Kind {
	case processOwned:
		return value.Contract == "agentlab.worker-result.v1"
	case processManaged:
		if state.started.Coder != nil {
			return value.Contract == coderResultContract && state.coderCompletion != nil
		}
		return value.Contract == managedResultContract
	default:
		return false
	}
}
