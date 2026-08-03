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
	if json.Unmarshal(record.Data, &value) != nil || s.exit != nil || !s.closedStreams["stdout"] || !s.closedStreams["stderr"] || s.started.Process.Kind != processOwned {
		return invalid(record, "invalid process_exited")
	}
	s.exit = &value
	return nil
}

func (s *replayState) terminal(record ledger.Record) error {
	if s.exit == nil {
		return invalid(record, "terminal fact before exit")
	}
	if record.Kind == eventTerminalAccepted {
		var value terminalResult
		if json.Unmarshal(record.Data, &value) != nil || value.Contract != "agentlab.worker-result.v1" || value.Outcome != "success" {
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
