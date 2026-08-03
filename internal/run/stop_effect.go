package run

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
)

type StopPayload struct {
	Reason string `json:"reason"`
}

type StopEffectResult struct {
	Stop    StopResult     `json:"stop"`
	Receipt effect.Receipt `json:"receipt"`
}

func EncodeStopPayload(value StopPayload) ([]byte, error) {
	if value.Reason == "" || len(value.Reason) > 4096 {
		return nil, errors.New("stop payload is invalid")
	}
	return json.Marshal(value)
}

func (o *Operation) RequestStopEffect(intent effect.Intent) (StopEffectResult, error) {
	if intent.Kind != effect.Stop || intent.RunID != o.runID || intent.Validate() != nil {
		return StopEffectResult{}, errors.New("stop effect intent is invalid")
	}
	payload, err := o.ReadEffectPayload(intent)
	if err != nil {
		return StopEffectResult{}, err
	}
	var value StopPayload
	if strictjson.Decode(payload, &value) != nil {
		return StopEffectResult{}, errors.New("stop payload is invalid")
	}
	if _, err := EncodeStopPayload(value); err != nil {
		return StopEffectResult{}, err
	}
	created, err := o.BeginEffectAttempt(intent, payload)
	if err != nil {
		return StopEffectResult{}, err
	}
	if !created {
		return o.reconcileStopEffect(intent, value)
	}
	return o.requestAndSettleStop(intent, value)
}

func (o *Operation) reconcileStopEffect(intent effect.Intent, payload StopPayload) (StopEffectResult, error) {
	evidence, exists, err := o.EffectObservation(intent)
	if err != nil {
		return StopEffectResult{}, err
	}
	if exists {
		return o.settleObservedStop(intent, evidence)
	}
	return o.requestAndSettleStop(intent, payload)
}

func (o *Operation) requestAndSettleStop(intent effect.Intent, payload StopPayload) (StopEffectResult, error) {
	result, err := o.RequestStop(payload.Reason)
	if err != nil || result.Reason != payload.Reason || !result.Admitted {
		return StopEffectResult{}, errors.New("stop effect is not durably admitted")
	}
	evidence, err := json.Marshal(result)
	if err != nil {
		return StopEffectResult{}, err
	}
	if err := o.RecordEffectObservation(intent, evidence); err != nil {
		return StopEffectResult{}, err
	}
	return o.settleObservedStop(intent, evidence)
}

func (o *Operation) settleObservedStop(intent effect.Intent, evidence []byte) (StopEffectResult, error) {
	var result StopResult
	if strictjson.Decode(evidence, &result) != nil || result.ID == "" || !result.Admitted {
		return StopEffectResult{}, errors.New("stop effect observation is invalid")
	}
	if receipt, exists, err := o.EffectReceipt(intent.ID); err != nil {
		return StopEffectResult{}, err
	} else if exists {
		return StopEffectResult{Stop: result, Receipt: receipt}, nil
	}
	receipt, err := o.SettleEffect(intent, evidence)
	return StopEffectResult{Stop: result, Receipt: receipt}, err
}
