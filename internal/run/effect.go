package run

import (
	"bytes"
	"errors"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/transaction"
)

// SettleEffect records the evidence that one already-admitted intent completed.
// It cannot create an intent; Experiment owns admission and gate reconciliation.
func (o *Operation) SettleEffect(intent effect.Intent, evidence []byte) (effect.Receipt, error) {
	if intent.RunID != o.runID || intent.Validate() != nil || len(evidence) == 0 {
		return effect.Receipt{}, errors.New("effect settlement is invalid")
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return effect.Receipt{}, err
	}
	defer lease.Release()
	state, err := o.currentState()
	if err != nil || state.started == nil || state.terminalSeen {
		return effect.Receipt{}, errors.New("run cannot settle effect")
	}
	if _, exists := state.effectReceipts[intent.ID]; exists {
		return effect.Receipt{}, errors.New("effect intent is already settled")
	}
	if _, err := o.ReadEffectPayload(intent); err != nil {
		return effect.Receipt{}, err
	}
	if intent.Kind == effect.Checkpoint || intent.Kind == effect.Fork {
		observation, observed, err := o.EffectObservation(intent)
		if err != nil || !observed || !bytes.Equal(observation, evidence) {
			return effect.Receipt{}, errors.New("runtime effect receipt has no matching prior observation")
		}
	}
	ref, err := o.artifacts.Put(evidence)
	if err != nil {
		return effect.Receipt{}, err
	}
	receipt := effect.Receipt{IntentID: intent.ID, Kind: intent.Kind, Evidence: ref}
	if _, err := o.appendEvent(time.Now().UTC(), eventEffectReceipt, effectReceiptRecorded{Receipt: receipt}); err != nil {
		return effect.Receipt{}, err
	}
	return receipt, nil
}

func (o *Operation) ReadEffectPayload(intent effect.Intent) ([]byte, error) {
	if intent.RunID != o.runID || intent.Validate() != nil {
		return nil, errors.New("effect intent is invalid")
	}
	return o.artifacts.Read(intent.Payload)
}

func (o *Operation) EffectReceipts() ([]effect.Receipt, error) {
	state, err := o.currentState()
	if err != nil {
		return nil, err
	}
	result := make([]effect.Receipt, 0, len(state.effectReceipts))
	for _, receipt := range state.effectReceipts {
		result = append(result, receipt)
	}
	return result, nil
}

func (o *Operation) EffectReceipt(intentID string) (effect.Receipt, bool, error) {
	state, err := o.currentState()
	if err != nil {
		return effect.Receipt{}, false, err
	}
	receipt, exists := state.effectReceipts[intentID]
	return receipt, exists, nil
}
