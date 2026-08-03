package experiment

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

const FixtureResetContract = "agentlab.fixture-reset-proof.v1"

type FixtureResetProof struct {
	Contract string         `json:"contract"`
	RunID    string         `json:"run_id"`
	Fixture  artifact.Ref   `json:"fixture"`
	Baseline artifact.Ref   `json:"baseline"`
	Evidence []artifact.Ref `json:"evidence"`
}

func RecordFixtureReset(store artifact.Store, proof FixtureResetProof) (artifact.Ref, error) {
	if err := validateFixtureReset(store, proof); err != nil {
		return artifact.Ref{}, err
	}
	data, err := json.Marshal(proof)
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.PutCanonicalJSON(data)
}

func loadFixtureReset(store artifact.Store, ref artifact.Ref) (FixtureResetProof, error) {
	data, err := store.Read(ref)
	if err != nil {
		return FixtureResetProof{}, err
	}
	var proof FixtureResetProof
	if err := decode(data, &proof); err != nil {
		return FixtureResetProof{}, err
	}
	if err := validateFixtureReset(store, proof); err != nil {
		return FixtureResetProof{}, err
	}
	return proof, nil
}

func validateFixtureReset(store artifact.Store, proof FixtureResetProof) error {
	if proof.Contract != FixtureResetContract || !idPattern.MatchString(proof.RunID) || !validRef(proof.Fixture) || !validRef(proof.Baseline) || len(proof.Evidence) == 0 || len(proof.Evidence) > 100 {
		return errors.New("fixture reset proof is invalid")
	}
	seen := map[artifact.Ref]bool{}
	for _, ref := range append([]artifact.Ref{proof.Fixture, proof.Baseline}, proof.Evidence...) {
		if !validRef(ref) || seen[ref] {
			return errors.New("fixture reset proof references are invalid or duplicated")
		}
		seen[ref] = true
		if _, err := store.Read(ref); err != nil {
			return err
		}
	}
	return nil
}
