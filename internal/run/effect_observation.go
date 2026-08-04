package run

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/transaction"
)

// RecordEffectObservation stores an exact post-effect observation before the
// receipt. Recovery may settle this observation but may not repeat the effect.
func (o *Operation) RecordEffectObservation(intent effect.Intent, data []byte) error {
	if intent.RunID != o.runID || intent.Validate() != nil || len(data) == 0 || len(data) > 64*1024 {
		return errors.New("effect observation is invalid")
	}
	return transaction.WriteOnce(filepath.Join(o.dir, "effect-observations", intent.ID+".json"), data, 0o600)
}

func (o *Operation) EffectObservation(intent effect.Intent) ([]byte, bool, error) {
	if intent.RunID != o.runID || intent.Validate() != nil {
		return nil, false, errors.New("effect intent is invalid")
	}
	data, err := os.ReadFile(filepath.Join(o.dir, "effect-observations", intent.ID+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

// verifyObservedReceipt proves that the durable receipt is the exact
// post-effect observation. Runtime-specific verifiers add their own state
// checks; generic settlement alone intentionally does not assert execution.
func (o *Operation) verifyObservedReceipt(intent effect.Intent) ([]byte, effect.Receipt, error) {
	observation, observed, err := o.EffectObservation(intent)
	if err != nil || !observed {
		return nil, effect.Receipt{}, errors.New("runtime effect observation is absent")
	}
	receipt, settled, err := o.EffectReceipt(intent.ID)
	if err != nil || !settled || receipt.IntentID != intent.ID || receipt.Kind != intent.Kind {
		return nil, effect.Receipt{}, errors.New("runtime effect receipt is unavailable")
	}
	evidence, err := o.artifacts.Read(receipt.Evidence)
	if err != nil || !bytes.Equal(observation, evidence) {
		return nil, effect.Receipt{}, errors.New("runtime effect receipt differs from observation")
	}
	return observation, receipt, nil
}
