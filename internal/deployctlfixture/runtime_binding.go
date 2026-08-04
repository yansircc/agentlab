package deployctlfixture

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const runtimeBindingContract = "agentlab.deployctl-worker-runtime.v1"

// runtimeBinding is the evaluated manifest's opaque bridge to Host-private
// profiles. It has refs and profile names, never a path or executable value.
type runtimeBinding struct {
	Contract            string       `json:"contract"`
	Adapter             artifact.Ref `json:"adapter"`
	CandidateExecutable artifact.Ref `json:"candidate_executable"`
	WorkerProfile       string       `json:"worker_profile"`
	CoderProfile        string       `json:"coder_profile"`
}

func (value runtimeBinding) validate() error {
	if value.Contract != runtimeBindingContract || !value.Adapter.Valid() || !value.CandidateExecutable.Valid() || value.WorkerProfile != "baseline-worker" || value.CoderProfile != "coder-repair" {
		return errors.New("deployctl runtime binding is invalid")
	}
	return nil
}

func recordRuntimeBinding(store artifact.Store, value runtimeBinding) (artifact.Ref, error) {
	if value.validate() != nil {
		return artifact.Ref{}, errors.New("deployctl runtime binding is invalid")
	}
	return putCanonical(store, value)
}

func loadRuntimeBinding(store artifact.Store, ref artifact.Ref) (runtimeBinding, error) {
	data, err := store.Read(ref)
	if err != nil {
		return runtimeBinding{}, err
	}
	var value runtimeBinding
	if strictjson.Decode(data, &value) != nil || value.validate() != nil {
		return runtimeBinding{}, errors.New("deployctl runtime binding is invalid")
	}
	return value, nil
}
