package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/transaction"
)

const effectAttemptContract = "agentlab.effect-attempt.v1"

type effectAttempt struct {
	Contract string        `json:"contract"`
	Intent   effect.Intent `json:"intent"`
	State    []byte        `json:"state"`
}

// BeginEffectAttempt durably marks an effect as unknown before its external
// execution. A later caller must reconcile that one attempt, never retry it.
func (o *Operation) BeginEffectAttempt(intent effect.Intent, state []byte) (created bool, err error) {
	if intent.RunID != o.runID || intent.Validate() != nil || len(state) == 0 || len(state) > 64*1024 {
		return false, errors.New("effect attempt is invalid")
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return false, err
	}
	defer func() {
		if releaseErr := lease.Release(); err == nil && releaseErr != nil {
			err = releaseErr
		}
	}()
	path := filepath.Join(o.dir, "effect-attempts", intent.ID+".json")
	data, readErr := os.ReadFile(path)
	if errors.Is(readErr, os.ErrNotExist) {
		data, err = json.Marshal(effectAttempt{Contract: effectAttemptContract, Intent: intent, State: state})
		if err != nil {
			return false, err
		}
		if err := transaction.WriteOnce(path, data, 0o600); err != nil {
			return false, err
		}
		return true, nil
	}
	if readErr != nil {
		return false, readErr
	}
	value, err := decodeEffectAttempt(data)
	if err != nil || value.Intent != intent || !bytes.Equal(value.State, state) {
		return false, errors.New("effect attempt identity changed")
	}
	return false, nil
}

func decodeEffectAttempt(data []byte) (effectAttempt, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value effectAttempt
	if err := decoder.Decode(&value); err != nil || value.Contract != effectAttemptContract || value.Intent.Validate() != nil || len(value.State) == 0 {
		return effectAttempt{}, errors.New("effect attempt is corrupt")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return effectAttempt{}, errors.New("effect attempt has trailing input")
	}
	return value, nil
}
