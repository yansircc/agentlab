package pi

import (
	"bytes"
	"errors"
	"reflect"

	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

// ReconciledFork is Host-private recovery output. Its child path is derived
// from the adapter-owned session directory and never enters a tool response.
type ReconciledFork struct {
	Forked           run.SessionForked
	ChildSessionPath string
}

// ReconcileForkedSession recovers exactly one settled child. It rechecks the
// bundled runtime, parent checkpoint, child parent link and public prefix; it
// never guesses another session or repeats the SDK fork.
func ReconcileForkedSession(operation *run.Operation, intentID string, spec ForkSpec, config IdentityConfig) (ReconciledFork, error) {
	identity, err := VerifyRuntimeIdentity(config)
	if err != nil {
		return ReconciledFork{}, err
	}
	forked, childData, _, err := operation.ForkReceipt(intentID)
	if err != nil {
		return ReconciledFork{}, err
	}
	checkpoint, prefix, session, opaque, err := operation.RuntimeCheckpointData(forked.ExpectedCheckpoint)
	if err != nil || checkpoint.Adapter != adapterName || forked.ParentSession != checkpoint.Session {
		return ReconciledFork{}, errors.New("Pi fork receipt does not match checkpoint")
	}
	prefixRef, err := operation.RuntimeCheckpointPublicPrefix(forked.ExpectedCheckpoint)
	if err != nil || forked.ObservedPrefix != prefixRef {
		return ReconciledFork{}, errors.New("Pi fork receipt prefix differs from checkpoint")
	}
	state, err := validateForkParent(spec, checkpoint, session, opaque, prefix, identity)
	if err != nil {
		return ReconciledFork{}, err
	}
	attempt, err := newForkAttempt(spec, state, checkpoint, forked.ExpectedCheckpoint, identity)
	if err != nil {
		return ReconciledFork{}, err
	}
	path, err := reconcileFork(attempt)
	if err != nil {
		return ReconciledFork{}, err
	}
	child, observed, err := validateForkChild(path, attempt)
	if err != nil || !bytes.Equal(observed, prefix) {
		return ReconciledFork{}, errors.New("Pi reconciled child prefix differs")
	}
	var receipt sessionReceipt
	if strictjson.Decode(childData, &receipt) != nil || receipt.Contract != checkpointSessionContract || !reflect.DeepEqual(receipt.Identity, identity) || receipt.RuntimeLocator != child.RuntimeLocator {
		return ReconciledFork{}, errors.New("Pi reconciled child session differs from receipt")
	}
	return ReconciledFork{Forked: forked, ChildSessionPath: path}, nil
}
