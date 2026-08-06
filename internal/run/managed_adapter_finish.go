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

// acquireProducerLease records a durable terminal or completion fact. A
// managed process can exit while the same Host is still committing adapter
// evidence under the producer lease; the terminal fact must wait briefly for
// that live writer instead of silently failing closed. The bound still fails
// closed on a stale or stuck lease.
func acquireProducerLease(dir string) (*transaction.Lease, error) {
	path := filepath.Join(dir, "producer.lock")
	const attempts = 750 // ~15s of 20ms backoff, far beyond any evidence commit
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		var lease *transaction.Lease
		lease, err = transaction.Acquire(path)
		if err == nil {
			return lease, nil
		}
		if !errors.Is(err, transaction.ErrLeaseHeld) {
			return nil, err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, err
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
	lease, err := acquireProducerLease(o.dir)
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
