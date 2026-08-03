package run

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
)

type StartPayload struct {
	Handoff *artifact.Ref `json:"handoff,omitempty"`
}

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

func EncodeStartPayload(kind effect.Kind, value StartPayload) ([]byte, error) {
	if (kind == effect.WorkerStart && value.Handoff != nil) || (kind == effect.CoderStart && (value.Handoff == nil || !validRef(*value.Handoff))) || (kind != effect.WorkerStart && kind != effect.CoderStart) {
		return nil, errors.New("start payload is invalid")
	}
	return json.Marshal(value)
}

func (o *Operation) BeginAttachedEffect(intent effect.Intent, spec AttachedSpec) (AttachedStartResult, error) {
	if intent.RunID != o.runID || (intent.Kind != effect.WorkerStart && intent.Kind != effect.CoderStart) || intent.Validate() != nil || validateAttachedSpec(spec) != nil {
		return AttachedStartResult{}, errors.New("attached start effect is invalid")
	}
	payload, err := o.ReadEffectPayload(intent)
	if err != nil {
		return AttachedStartResult{}, err
	}
	var start StartPayload
	if strictjson.Decode(payload, &start) != nil {
		return AttachedStartResult{}, errors.New("start payload is invalid")
	}
	if _, err := EncodeStartPayload(intent.Kind, start); err != nil {
		return AttachedStartResult{}, err
	}
	if start.Handoff != nil {
		if _, err := o.artifacts.Read(*start.Handoff); err != nil {
			return AttachedStartResult{}, err
		}
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
		return o.reconcileAttachedStart(intent, spec)
	}
	state, err := o.BeginAttached(spec)
	if err != nil {
		return AttachedStartResult{}, err
	}
	evidence, err := json.Marshal(state)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if err := o.RecordEffectObservation(intent, evidence); err != nil {
		return AttachedStartResult{}, err
	}
	return o.settleAttachedStart(intent, evidence)
}

func (o *Operation) reconcileAttachedStart(intent effect.Intent, spec AttachedSpec) (AttachedStartResult, error) {
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
	evidence, err = json.Marshal(state)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if err := o.RecordEffectObservation(intent, evidence); err != nil {
		return AttachedStartResult{}, err
	}
	return o.settleAttachedStart(intent, evidence)
}

func (o *Operation) settleAttachedStart(intent effect.Intent, evidence []byte) (AttachedStartResult, error) {
	var state AdapterState
	if strictjson.Decode(evidence, &state) != nil || state.Adapter == "" || state.StreamID == "" {
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
