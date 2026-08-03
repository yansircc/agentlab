package run

import (
	"encoding/json"

	"github.com/yansircc/agentlab/internal/ledger"
)

func (s *replayState) effectReceipt(record ledger.Record) error {
	var value effectReceiptRecorded
	if s.started == nil || s.terminalSeen || json.Unmarshal(record.Data, &value) != nil || value.Receipt.Validate() != nil {
		return invalid(record, "invalid effect receipt")
	}
	if _, exists := s.effectReceipts[value.Receipt.IntentID]; exists {
		return invalid(record, "duplicate effect receipt")
	}
	s.effectReceipts[value.Receipt.IntentID] = value.Receipt
	return nil
}
