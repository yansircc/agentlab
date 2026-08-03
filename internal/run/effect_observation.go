package run

import (
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
