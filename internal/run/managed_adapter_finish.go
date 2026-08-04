package run

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/transaction"
)

func (o *Operation) awaitManaged(command *exec.Cmd, finalize func(int) error) {
	code, err := managedExitCode(command.Wait())
	if err == nil {
		err = finalize(code)
	}
	_ = o.finishManaged(code, err)
}

func managedExitCode(waitErr error) (int, error) {
	if waitErr == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, waitErr
}

func (o *Operation) finishManaged(code int, completionErr error) error {
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return err
	}
	defer lease.Release()
	state, err := o.currentState()
	if err != nil || state.started == nil || state.started.Process.Kind != processManaged || state.exit != nil || state.terminalSeen {
		return errors.New("managed completion is not admissible")
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventProcessExited, processExited{Code: code}); err != nil {
		return err
	}
	if completionErr != nil {
		_, err = o.appendEvent(time.Now().UTC(), eventTerminalRejected, terminalRejected{Reason: completionErr.Error()})
		return err
	}
	if code != 0 {
		_, err = o.appendEvent(time.Now().UTC(), eventTerminalRejected, terminalRejected{Reason: fmt.Sprintf("managed process exited with code %d", code)})
		return err
	}
	contract := managedResultContract
	if state.started.Coder != nil {
		if state.coderCompletion == nil {
			_, err = o.appendEvent(time.Now().UTC(), eventTerminalRejected, terminalRejected{Reason: "coder completion is absent"})
			return err
		}
		contract = coderResultContract
	}
	_, err = o.appendEvent(time.Now().UTC(), eventTerminalAccepted, terminalResult{Contract: contract, Outcome: "success"})
	return err
}
