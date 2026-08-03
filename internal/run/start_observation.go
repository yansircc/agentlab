package run

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
)

type startObservation struct {
	State AdapterState  `json:"state"`
	Coder *CoderProfile `json:"coder,omitempty"`
}

func encodeStartObservation(state AdapterState, payload StartPayload) ([]byte, error) {
	if state.Adapter == "" || state.StreamID == "" {
		return nil, errors.New("start state is invalid")
	}
	return json.Marshal(startObservation{State: state, Coder: payload.Coder})
}

func decodeStartObservation(data []byte, kind effect.Kind) (AdapterState, error) {
	var value startObservation
	if strictjson.Decode(data, &value) != nil || value.State.Adapter == "" || value.State.StreamID == "" {
		return AdapterState{}, errors.New("start observation is invalid")
	}
	if (kind == effect.WorkerStart && value.Coder != nil) || (kind == effect.CoderStart && (value.Coder == nil || value.Coder.Validate() != nil)) {
		return AdapterState{}, errors.New("start observation role differs")
	}
	return value.State, nil
}
