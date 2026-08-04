package tool

import "encoding/json"

// EncodePiRuntimePlan is the sole Host-owned wire encoder for the runtime
// plan consumed by the bundled extension. The provider never receives its
// paths; it can select only a pre-registered opaque profile ref.
func EncodePiRuntimePlan(profiles []PiRuntimeProfile) ([]byte, error) {
	if _, err := NewPiRuntimeHost(profiles); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Contract string             `json:"contract"`
		Profiles []PiRuntimeProfile `json:"profiles"`
	}{Contract: piRuntimePlanContract, Profiles: profiles})
}
