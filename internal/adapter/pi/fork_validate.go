package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"

	"github.com/yansircc/agentlab/internal/run"
)

func validateForkParent(spec ForkSpec, checkpoint run.RuntimeCheckpoint, sessionData, opaque, prefix []byte, identity AdapterIdentity) (checkpointState, error) {
	var session sessionReceipt
	var state checkpointState
	if decodeForkJSON(sessionData, &session) != nil || decodeForkJSON(opaque, &state) != nil || session.Contract != checkpointSessionContract || session.Identity.Validate() != nil || !reflect.DeepEqual(session.Identity, identity) || session.RuntimeLocator == "" {
		return checkpointState{}, errors.New("Pi checkpoint session receipt is invalid")
	}
	parent, err := ReadPublicTree(spec.ParentSession)
	if err != nil || parent.RuntimeLocator != session.RuntimeLocator || parent.SessionID() != state.SessionID || state.PrefixDigest != checkpoint.PrefixDigest {
		return checkpointState{}, errors.New("Pi parent session does not match checkpoint")
	}
	actual, _, digest, err := parent.Checkpoint(opaqueLocator(parent.SessionID(), state.EntryID))
	if err != nil || digest != state.PrefixDigest || !bytes.Equal(actual, prefix) {
		return checkpointState{}, errors.New("Pi parent public prefix does not match checkpoint")
	}
	return state, nil
}

func validateForkChild(path string, attempt forkAttempt) (PublicTree, []byte, error) {
	parent, err := readSessionParent(path)
	if err != nil || !samePath(parent, attempt.ParentSession) {
		return PublicTree{}, nil, errors.New("Pi child does not link to parent session")
	}
	child, err := ReadPublicTree(path)
	if err != nil {
		return PublicTree{}, nil, err
	}
	prefix, _, digest, err := child.Checkpoint(opaqueLocator(child.SessionID(), attempt.EntryID))
	if err != nil || digest != attempt.PrefixDigest {
		return PublicTree{}, nil, errors.New("Pi child checkpoint is invalid")
	}
	return child, prefix, nil
}

func samePath(left, right string) bool {
	leftPath, leftErr := filepath.EvalSymlinks(left)
	rightPath, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && leftPath == rightPath
}

func decodeForkJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Pi JSON has trailing input")
	}
	return nil
}
