package pi

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

const checkpointSessionContract = "agentlab.pi-session.v1"

type CheckpointResult struct {
	Checkpoint   run.RuntimeCheckpointRecord `json:"checkpoint"`
	Runtime      string                      `json:"runtime_locator"`
	EntryLocator string                      `json:"entry_locator"`
	PrefixDigest string                      `json:"prefix_digest"`
}

func Checkpoint(operation *run.Operation, intent effect.Intent, sessionPath, entryLocator string, evidence run.EvidenceRef, expectedPrefix string, identity AdapterIdentity) (CheckpointResult, error) {
	if err := identity.Validate(); err != nil || intent.Validate() != nil || intent.Kind != effect.Checkpoint || expectedPrefix == "" {
		return CheckpointResult{}, errors.New("Pi checkpoint request is invalid")
	}
	evidenceItem, err := operation.EvidenceAt(evidence)
	if err != nil {
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
	var selected PublicEntry
	for _, entry := range tree.Entries {
		if entry.Locator == entryLocator {
			selected = entry
			break
		}
	}
	source, err := selected.EvidenceSource()
	if err != nil || evidenceItem.SourceLocator != source {
		return CheckpointResult{}, errors.New("Pi checkpoint evidence differs from selected entry")
	}
	prefix, opaque, digest, err := tree.Checkpoint(entryLocator)
	if err != nil {
		return CheckpointResult{}, err
	}
	if digest != expectedPrefix {
		return CheckpointResult{}, errors.New("Pi checkpoint prefix differs from durable selection")
	}
	session, err := json.Marshal(sessionReceipt{Contract: checkpointSessionContract, RuntimeLocator: tree.RuntimeLocator, Identity: identity})
	if err != nil {
		return CheckpointResult{}, err
	}
	record, err := operation.RecordRuntimeCheckpoint(intent, run.RuntimeCheckpointSpec{
		Adapter: adapterName, Session: session, OpaqueState: opaque, PublicPrefix: prefix,
	})
	if err != nil || record.PublicPrefix.Digest != digest {
		return CheckpointResult{}, errors.New("Pi checkpoint receipt prefix does not match public tree")
	}
	return CheckpointResult{Checkpoint: record, Runtime: tree.RuntimeLocator, EntryLocator: entryLocator, PrefixDigest: digest}, nil
}
