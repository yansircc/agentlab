package comparison

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/strictjson"
)

// EncodeOracleEvidence is for Host adapters and objective harnesses. Provider
// tools do not expose a path to write this artifact.
func EncodeOracleEvidence(value OracleEvidence) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return artifact.CanonicalJSON(data)
}

func LoadOracleEvidence(store artifact.Store, ref artifact.Ref) (OracleEvidence, error) {
	if !ref.Valid() {
		return OracleEvidence{}, errors.New("oracle evidence reference is invalid")
	}
	data, err := store.Read(ref)
	if err != nil {
		return OracleEvidence{}, err
	}
	canonical, err := artifact.CanonicalJSON(data)
	if err != nil || !bytes.Equal(data, canonical) {
		return OracleEvidence{}, errors.New("oracle evidence is not canonical")
	}
	var value OracleEvidence
	if strictjson.Decode(data, &value) != nil || value.Validate() != nil {
		return OracleEvidence{}, errors.New("oracle evidence is invalid")
	}
	return value, nil
}
