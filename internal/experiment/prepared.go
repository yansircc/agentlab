package experiment

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const PreparedRunContract = "agentlab.prepared-run.v1"

// PreparedRun is the Host-issued complete input set for one future manifest.
// It has no origin because checkpoint choice remains the Supervisor's claim.
type PreparedRun struct {
	Contract string    `json:"contract"`
	RunID    string    `json:"run_id"`
	Inputs   RunInputs `json:"inputs"`
}

func RecordPreparedRun(store artifact.Store, value PreparedRun) (artifact.Ref, error) {
	if err := validatePreparedRun(store, value); err != nil {
		return artifact.Ref{}, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.PutCanonicalJSON(data)
}

func LoadPreparedRun(store artifact.Store, ref artifact.Ref) (PreparedRun, error) {
	data, err := store.Read(ref)
	if err != nil {
		return PreparedRun{}, err
	}
	var value PreparedRun
	if strictjson.Decode(data, &value) != nil || validatePreparedRun(store, value) != nil {
		return PreparedRun{}, errors.New("prepared run is invalid")
	}
	return value, nil
}

func validatePreparedRun(store artifact.Store, value PreparedRun) error {
	if value.Contract != PreparedRunContract || !idPattern.MatchString(value.RunID) {
		return errors.New("prepared run is invalid")
	}
	for _, ref := range inputRefs(value.Inputs) {
		if !validRef(ref) {
			return errors.New("prepared run inputs are invalid")
		}
		if _, err := store.Read(ref); err != nil {
			return err
		}
	}
	if _, err := source.Load(store, value.Inputs.Candidate); err != nil {
		return errors.New("prepared run candidate is invalid")
	}
	reset, err := loadFixtureReset(store, value.Inputs.FixtureReset)
	if err != nil || reset.RunID != value.RunID || reset.Fixture != value.Inputs.Fixture {
		return errors.New("prepared run fixture reset is invalid")
	}
	return nil
}
