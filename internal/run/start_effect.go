package run

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
)

type AttachedStartResult struct {
	State   AdapterState   `json:"state"`
	Receipt effect.Receipt `json:"receipt"`
}

type attachedStartAttempt struct {
	Adapter      string              `json:"adapter"`
	StreamID     string              `json:"stream_id"`
	Policy       StopPolicy          `json:"policy"`
	Capabilities AdapterCapabilities `json:"capabilities"`
}

func (o *Operation) BeginAttachedEffect(intent effect.Intent, spec AttachedSpec) (AttachedStartResult, error) {
	if intent.RunID != o.runID || (intent.Kind != effect.WorkerStart && intent.Kind != effect.CoderStart) || intent.Validate() != nil || validateAttachedSpec(spec) != nil {
		return AttachedStartResult{}, errors.New("attached start effect is invalid")
	}
	start, err := o.startPayload(intent)
	if err != nil {
		return AttachedStartResult{}, err
	}
	attempt, err := json.Marshal(attachedStartAttempt{Adapter: spec.Adapter, StreamID: spec.StreamID, Policy: spec.Policy, Capabilities: spec.Capabilities})
	if err != nil {
		return AttachedStartResult{}, err
	}
	created, err := o.BeginEffectAttempt(intent, attempt)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if !created {
		return o.reconcileAttachedStart(intent, spec, start)
	}
	state, err := o.BeginAttached(spec)
	if err != nil {
		return AttachedStartResult{}, err
	}
	evidence, err := encodeStartObservation(state, start)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if err := o.RecordEffectObservation(intent, evidence); err != nil {
		return AttachedStartResult{}, err
	}
	return o.settleAttachedStart(intent, evidence)
}

func (o *Operation) startPayload(intent effect.Intent) (StartPayload, error) {
	payload, err := o.ReadEffectPayload(intent)
	if err != nil {
		return StartPayload{}, err
	}
	var value StartPayload
	if strictjson.Decode(payload, &value) != nil {
		return StartPayload{}, errors.New("start payload is invalid")
	}
	if _, err := EncodeStartPayload(intent.Kind, value); err != nil {
		return StartPayload{}, err
	}
	if value.Coder != nil {
		for _, ref := range value.Coder.refs() {
			if _, err := o.artifacts.Read(ref); err != nil {
				return StartPayload{}, err
			}
		}
	}
	return value, nil
}

func (o *Operation) reconcileAttachedStart(intent effect.Intent, spec AttachedSpec, payload StartPayload) (AttachedStartResult, error) {
	evidence, exists, err := o.EffectObservation(intent)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if exists {
		return o.settleAttachedStart(intent, evidence)
	}
	state, err := o.AdapterState(spec.Adapter)
	if err != nil || state.StreamID != spec.StreamID {
		return AttachedStartResult{}, errors.New("attached start outcome is unknown; refusing to repeat it")
	}
	evidence, err = encodeStartObservation(state, payload)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if err := o.RecordEffectObservation(intent, evidence); err != nil {
		return AttachedStartResult{}, err
	}
	return o.settleAttachedStart(intent, evidence)
}

func (o *Operation) settleAttachedStart(intent effect.Intent, evidence []byte) (AttachedStartResult, error) {
	state, err := decodeStartObservation(evidence, intent.Kind)
	if err != nil {
		return AttachedStartResult{}, errors.New("attached start observation is invalid")
	}
	if receipt, exists, err := o.EffectReceipt(intent.ID); err != nil {
		return AttachedStartResult{}, err
	} else if exists {
		return AttachedStartResult{State: state, Receipt: receipt}, nil
	}
	receipt, err := o.SettleEffect(intent, evidence)
	return AttachedStartResult{State: state, Receipt: receipt}, err
}

// VerifyStartEffect proves that a settled start receipt came from the one
// durable observation of the run's actual process-start state. It is used by
// the recursive gate; generic effect settlement alone is insufficient because
// a receipt does not by itself establish that the start effect was executed.
func (o *Operation) VerifyStartEffect(intent effect.Intent) error {
	if intent.RunID != o.runID || (intent.Kind != effect.WorkerStart && intent.Kind != effect.CoderStart) || intent.Validate() != nil {
		return errors.New("start effect intent is invalid")
	}
	payload, err := o.startPayload(intent)
	if err != nil {
		return err
	}
	observation, exists, err := o.EffectObservation(intent)
	if err != nil || !exists {
		return errors.New("start effect observation is absent")
	}
	state, err := decodeStartObservation(observation, intent.Kind)
	if err != nil {
		return err
	}
	current, err := o.currentState()
	if err != nil || current.started == nil || current.started.Adapter == nil || current.started.Adapter.Adapter != state.Adapter || current.started.Adapter.StreamID != state.StreamID || state.Stopped {
		return errors.New("start effect observation differs from run")
	}
	if intent.Kind == effect.CoderStart {
		if payload.Coder == nil || current.started.Coder == nil || *payload.Coder != *current.started.Coder {
			return errors.New("start effect Coder profile differs from run")
		}
	} else if payload.Coder != nil || current.started.Coder != nil {
		return errors.New("start effect Worker profile differs from run")
	}
	receipt, exists, err := o.EffectReceipt(intent.ID)
	if err != nil || !exists || receipt.IntentID != intent.ID || receipt.Kind != intent.Kind {
		return errors.New("start effect is not settled")
	}
	evidence, err := o.artifacts.Read(receipt.Evidence)
	if err != nil || !bytes.Equal(evidence, observation) {
		return errors.New("start receipt differs from observation")
	}
	return nil
}
