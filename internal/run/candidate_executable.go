package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const candidateExecutableContract = "agentlab.candidate-executable.v1"

// CandidateExecutable is the immutable bridge from a sealed source candidate
// to the exact executable bytes a Worker is allowed to invoke.
type CandidateExecutable struct {
	Contract   string       `json:"contract"`
	Candidate  artifact.Ref `json:"candidate"`
	Executable artifact.Ref `json:"executable"`
}

func BindCandidateExecutable(store artifact.Store, candidate, executable artifact.Ref) (artifact.Ref, error) {
	if !candidate.Valid() || !executable.Valid() {
		return artifact.Ref{}, errors.New("candidate executable refs are invalid")
	}
	if _, err := source.Load(store, candidate); err != nil {
		return artifact.Ref{}, err
	}
	if data, err := store.Read(executable); err != nil || len(data) == 0 {
		return artifact.Ref{}, errors.New("candidate executable bytes are invalid")
	}
	data, err := json.Marshal(CandidateExecutable{Contract: candidateExecutableContract, Candidate: candidate, Executable: executable})
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.PutCanonicalJSON(data)
}

func VerifyCandidateExecutable(store artifact.Store, receipt, candidate artifact.Ref, executable string) error {
	if !receipt.Valid() || !candidate.Valid() || !filepath.IsAbs(executable) {
		return errors.New("candidate executable verification is invalid")
	}
	data, err := store.Read(receipt)
	if err != nil {
		return err
	}
	var value CandidateExecutable
	if strictjson.Decode(data, &value) != nil || value.Contract != candidateExecutableContract || value.Candidate != candidate || !value.Executable.Valid() {
		return errors.New("candidate executable receipt differs")
	}
	want, err := store.Read(value.Executable)
	if err != nil {
		return err
	}
	got, err := os.ReadFile(executable)
	if err != nil || !bytes.Equal(got, want) {
		return errors.New("candidate executable bytes differ")
	}
	return nil
}
