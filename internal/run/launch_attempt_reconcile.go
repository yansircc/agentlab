package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/processidentity"
)

func (o *Operation) reconcileLaunchAttempts() error {
	root := filepath.Join(o.dir, "launch-attempts")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !attemptIDPattern.MatchString(entry.Name()) {
			return fmt.Errorf("invalid launch attempt entry %q", entry.Name())
		}
		attempt := &launchAttempt{id: entry.Name(), log: ledgerForAttempt(root, entry.Name())}
		state, err := attempt.state()
		if err != nil {
			return err
		}
		if state.terminated {
			continue
		}
		if state.identity == nil {
			return fmt.Errorf("%w: attempt %s may have spawned before identity persistence", ErrAttemptUnresolved, attempt.id)
		}
		observation := o.attemptProber.Observe(*state.identity)
		switch observation {
		case processidentity.Dead, processidentity.Mismatch:
			if err := attempt.terminate("reconciled_dead", string(observation)); err != nil {
				return err
			}
		case processidentity.Matches:
			if err := o.terminateIdentity(*state.identity); err != nil {
				return fmt.Errorf("%w: attempt %s: %v", ErrAttemptUnresolved, attempt.id, err)
			}
			if err := attempt.terminate("reconciled_live_orphan", "identity_verified_then_process_group_killed"); err != nil {
				return err
			}
		case processidentity.Unknown:
			return fmt.Errorf("%w: attempt %s identity probe unknown", ErrAttemptUnresolved, attempt.id)
		}
	}
	return nil
}

func ledgerForAttempt(root, id string) *ledger.Ledger {
	return ledger.Open(filepath.Join(root, id, "events.jsonl"))
}

func terminateRecordedIdentity(identity processidentity.Identity) error {
	if (processidentity.SystemProber{}).Observe(identity) != processidentity.Matches {
		return errors.New("process identity changed before termination")
	}
	if err := syscall.Kill(-identity.PGID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		observation := (processidentity.SystemProber{}).Observe(identity)
		if observation == processidentity.Dead || observation == processidentity.Mismatch {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("process identity remains live or unverifiable after kill")
}
