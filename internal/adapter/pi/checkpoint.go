package pi

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/run"
)

const checkpointSessionContract = "agentlab.pi-session.v1"

type CheckpointResult struct {
	Checkpoint   run.RuntimeCheckpointRecord `json:"checkpoint"`
	Runtime      string                      `json:"runtime_locator"`
	EntryLocator string                      `json:"entry_locator"`
	PrefixDigest string                      `json:"prefix_digest"`
}

func Checkpoint(operation *run.Operation, sessionPath, entryLocator string, identity AdapterIdentity) (CheckpointResult, error) {
	if err := identity.Validate(); err != nil {
		return CheckpointResult{}, err
	}
	tree, err := ReadPublicTree(sessionPath)
	if err != nil {
		return CheckpointResult{}, err
	}
	state, err := operation.AdapterState(adapterName)
	if err != nil || state.StreamID != tree.SessionID() {
		return CheckpointResult{}, errors.New("Pi checkpoint session does not match attached run")
	}
	prefix, opaque, digest, err := tree.Checkpoint(entryLocator)
	if err != nil {
		return CheckpointResult{}, err
	}
	session, err := json.Marshal(struct {
		Contract       string          `json:"contract"`
		RuntimeLocator string          `json:"runtime_locator"`
		Identity       AdapterIdentity `json:"identity"`
	}{Contract: checkpointSessionContract, RuntimeLocator: tree.RuntimeLocator, Identity: identity})
	if err != nil {
		return CheckpointResult{}, err
	}
	record, err := operation.RecordRuntimeCheckpoint(run.RuntimeCheckpointSpec{
		Adapter: adapterName, Session: session, OpaqueState: opaque, PublicPrefix: prefix,
	})
	if err != nil || record.PublicPrefix.Digest != digest {
		return CheckpointResult{}, errors.New("Pi checkpoint receipt prefix does not match public tree")
	}
	return CheckpointResult{Checkpoint: record, Runtime: tree.RuntimeLocator, EntryLocator: entryLocator, PrefixDigest: digest}, nil
}
