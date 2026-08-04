package tool

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/strictjson"
)

type piRuntimePlan struct {
	Contract        string                    `json:"contract"`
	Profiles        []PiRuntimeProfile        `json:"profiles"`
	PreparedWorkers []PiPreparedWorkerRuntime `json:"prepared_workers"`
}

// EncodePiRuntimePlan is the sole Host-owned wire encoder for static runtime
// roles. The provider never receives its paths; it can select only a
// pre-registered opaque profile ref.
func EncodePiRuntimePlan(profiles []PiRuntimeProfile) ([]byte, error) {
	return encodePiRuntimePlan(profiles, nil)
}

// EncodePiRuntimePlanWithPreparedWorkers additionally carries Host-issued
// Worker templates whose eventual splice session is receipt-derived.
func EncodePiRuntimePlanWithPreparedWorkers(profiles []PiRuntimeProfile, preparedWorkers []PiPreparedWorkerRuntime) ([]byte, error) {
	return encodePiRuntimePlan(profiles, preparedWorkers)
}

func encodePiRuntimePlan(profiles []PiRuntimeProfile, preparedWorkers []PiPreparedWorkerRuntime) ([]byte, error) {
	if len(profiles) == 0 || len(profiles)+len(preparedWorkers) > 1000 {
		return nil, errors.New("Pi runtime plan is invalid")
	}
	if _, err := newPiRuntimeHost(profiles, preparedWorkers); err != nil {
		return nil, err
	}
	return json.Marshal(piRuntimePlan{Contract: piRuntimePlanContract, Profiles: profiles, PreparedWorkers: preparedWorkers})
}

func decodePiRuntimePlan(data []byte) (piRuntimePlan, error) {
	var plan piRuntimePlan
	if strictjson.Decode(data, &plan) != nil || plan.Contract != piRuntimePlanContract || len(plan.Profiles) == 0 || len(plan.Profiles)+len(plan.PreparedWorkers) > 1000 {
		return piRuntimePlan{}, errors.New("Pi runtime plan is invalid")
	}
	if _, err := newPiRuntimeHost(plan.Profiles, plan.PreparedWorkers); err != nil {
		return piRuntimePlan{}, errors.New("Pi runtime plan is invalid")
	}
	return plan, nil
}
