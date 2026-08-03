package run

import (
	"encoding/json"

	"github.com/yansircc/agentlab/internal/ledger"
)

func (s *replayState) sessionForked(record ledger.Record) error {
	var value SessionForked
	if s.started == nil || s.terminalSeen || json.Unmarshal(record.Data, &value) != nil || value.Validate() != nil {
		return invalid(record, "invalid session fork receipt")
	}
	if existing, exists := s.sessionForks[value.ChildSession]; exists && existing != value {
		return invalid(record, "child session has duplicate fork receipt")
	}
	s.sessionForks[value.ChildSession] = value
	return nil
}
