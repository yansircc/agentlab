package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type wireResult struct {
	Type     string          `json:"type"`
	Contract string          `json:"contract"`
	Outcome  string          `json:"outcome"`
	Value    json.RawMessage `json:"value,omitempty"`
}

func validateTerminal(code int, corrupt bool, candidates [][]byte) (terminalResult, error) {
	if code != 0 {
		return terminalResult{}, fmt.Errorf("worker exited with code %d", code)
	}
	if corrupt {
		return terminalResult{}, errors.New("worker stream is corrupt")
	}
	if len(candidates) != 1 {
		return terminalResult{}, fmt.Errorf("expected exactly one terminal result, got %d", len(candidates))
	}
	decoder := json.NewDecoder(bytes.NewReader(candidates[0]))
	decoder.DisallowUnknownFields()
	var wire wireResult
	if err := decoder.Decode(&wire); err != nil {
		return terminalResult{}, fmt.Errorf("decode terminal result: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return terminalResult{}, errors.New("terminal result contains trailing JSON")
	}
	if wire.Type != "result" || wire.Contract != "agentlab.worker-result.v1" || wire.Outcome != "success" {
		return terminalResult{}, errors.New("terminal result violates contract")
	}
	return terminalResult{Contract: wire.Contract, Outcome: wire.Outcome, Value: wire.Value}, nil
}
