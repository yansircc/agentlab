package run

import (
	"errors"
	"fmt"
)

func (o *Operation) resolveStartAppendError(worker *ownedWorker, attempt *launchAttempt, started processStarted, appendErr error) (*ownedWorker, error) {
	records, replayErr := o.ledger.Replay()
	if replayErr == nil && len(records) == 1 {
		state, stateErr := replayRun(records)
		if stateErr == nil && state.started != nil && sameAcceptedAttempt(*state.started, started) {
			return worker, nil
		}
		if stateErr != nil {
			replayErr = stateErr
		} else {
			replayErr = errors.New("run ledger contains a different accepted start")
		}
	}
	terminateUnrecorded(worker.command, started.Process.Identity.PGID)
	journalErr := attempt.terminate("start_not_admitted", "process_group_killed_and_waited")
	if replayErr != nil {
		return nil, errors.Join(appendErr, replayErr, journalErr)
	}
	if len(records) != 0 {
		return nil, errors.Join(appendErr, fmt.Errorf("unexpected run ledger length %d", len(records)), journalErr)
	}
	return nil, errors.Join(appendErr, journalErr)
}

func sameAcceptedAttempt(left, right processStarted) bool {
	return left.AttemptID == right.AttemptID && left.Manifest == right.Manifest && left.Process.Kind == right.Process.Kind && left.Process.Identity != nil && right.Process.Identity != nil && *left.Process.Identity == *right.Process.Identity && left.Policy == right.Policy
}
