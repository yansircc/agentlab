package run

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const (
	coderCompletionContract = "agentlab.coder-completion.v1"
	coderResultContract     = "agentlab.coder-result.v1"
	managedResultContract   = "agentlab.managed-result.v1"
)

// CoderCompletion is Host-authored provenance for the only source snapshot a
// Coder run may offer as a repair candidate. It contains no session bytes.
type CoderCompletion struct {
	Contract  string       `json:"contract"`
	Profile   CoderProfile `json:"profile"`
	SessionID string       `json:"session_id"`
	Candidate artifact.Ref `json:"candidate"`
}

type coderCompletionRecorded struct {
	Receipt artifact.Ref `json:"receipt"`
}

func (value CoderCompletion) Validate() error {
	if value.Contract != coderCompletionContract || value.Profile.Validate() != nil || value.SessionID == "" || len(value.SessionID) > 256 || !value.Candidate.Valid() {
		return errors.New("coder completion is invalid")
	}
	return nil
}

// RecordCoderCompletion seals the Host-owned workspace after final session
// drain. The started Coder profile and stream identity are read from the run
// ledger; callers cannot choose either fact.
func (o *Operation) RecordCoderCompletion(candidate artifact.Ref) (artifact.Ref, error) {
	if _, err := source.Load(o.artifacts, candidate); err != nil {
		return artifact.Ref{}, errors.New("coder completion candidate is not a source snapshot")
	}
	// The same live-writer wait as finishManaged: the Coder process can exit
	// while the Host is still committing adapter evidence under the lease.
	lease, err := acquireProducerLease(o.dir)
	if err != nil {
		return artifact.Ref{}, err
	}
	defer lease.Release()
	state, err := o.currentState()
	if err != nil || state.started == nil || state.exit != nil || state.stopRequested || state.coderCompletion != nil || state.started.Process.Kind != processManaged || state.started.Coder == nil || state.started.Adapter == nil {
		return artifact.Ref{}, errors.New("coder completion is not admissible")
	}
	completion := CoderCompletion{Contract: coderCompletionContract, Profile: *state.started.Coder, SessionID: state.started.Adapter.StreamID, Candidate: candidate}
	if completion.Validate() != nil {
		return artifact.Ref{}, errors.New("coder completion is invalid")
	}
	data, err := json.Marshal(completion)
	if err != nil {
		return artifact.Ref{}, err
	}
	receipt, err := o.artifacts.PutCanonicalJSON(data)
	if err != nil {
		return artifact.Ref{}, err
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventCoderCompleted, coderCompletionRecorded{Receipt: receipt}); err != nil {
		return artifact.Ref{}, err
	}
	return receipt, nil
}

// CoderCompletionReceipt loads exactly the completion admitted by this run.
// It rejects lookalike artifacts that were not recorded in its ledger.
func (o *Operation) CoderCompletionReceipt() (artifact.Ref, CoderCompletion, error) {
	state, err := o.currentState()
	if err != nil || state.coderCompletion == nil || state.started == nil || state.started.Coder == nil || state.started.Adapter == nil || state.exit == nil || state.exit.Code != 0 || !state.terminalAccepted || state.terminalRejected {
		return artifact.Ref{}, CoderCompletion{}, errors.New("coder completion is absent")
	}
	receipt := state.coderCompletion.Receipt
	data, err := o.artifacts.Read(receipt)
	if err != nil {
		return artifact.Ref{}, CoderCompletion{}, err
	}
	var completion CoderCompletion
	if strictjson.Decode(data, &completion) != nil || completion.Validate() != nil || completion.Profile != *state.started.Coder || completion.SessionID != state.started.Adapter.StreamID {
		return artifact.Ref{}, CoderCompletion{}, errors.New("coder completion receipt differs from run")
	}
	if _, err := source.Load(o.artifacts, completion.Candidate); err != nil {
		return artifact.Ref{}, CoderCompletion{}, errors.New("coder completion candidate is invalid")
	}
	return receipt, completion, nil
}

func (s *replayState) coderCompleted(record ledger.Record) error {
	var value coderCompletionRecorded
	if strictjson.Decode(record.Data, &value) != nil || !value.Receipt.Valid() || s.started == nil || s.started.Process.Kind != processManaged || s.started.Coder == nil || s.exit != nil || s.stopRequested || s.coderCompletion != nil {
		return invalid(record, "invalid coder completion")
	}
	s.coderCompletion = &value
	return nil
}
