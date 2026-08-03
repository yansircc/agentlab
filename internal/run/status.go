package run

import (
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
)

func (o *Operation) Status(prober processidentity.Prober) (Status, error) {
	return o.StatusAt(prober, time.Now().UTC())
}

func (o *Operation) StatusAt(prober processidentity.Prober, now time.Time) (Status, error) {
	records, err := o.ledger.Replay()
	if err != nil {
		return Status{}, err
	}
	state, err := replayRun(records)
	if err != nil {
		return Status{}, err
	}
	status := state.projectActivity(now, uint64(len(records)))
	if state.started == nil {
		return status, nil
	}
	if state.exit != nil {
		status.ProcessLiveness = ProcessDead
		switch {
		case state.stopRequested:
			status.Health = HealthAbandoned
		case state.exit.Code != 0:
			status.Health = HealthExitedError
		case state.terminalAccepted && !state.terminalRejected && status.StreamActivity == StreamClosed:
			status.Health = HealthExitedClean
		default:
			status.Health = HealthTerminalCorrupt
		}
		return status, nil
	}
	if state.started.Process.Identity == nil {
		status.ProcessLiveness = ProcessUnknown
		if state.stopRequested {
			status.Health = HealthAbandoned
		} else {
			status.Health = HealthUnverifiable
		}
		return status, nil
	}
	switch prober.Observe(*state.started.Process.Identity) {
	case processidentity.Matches:
		status.ProcessLiveness = ProcessAlive
		if status.StreamActivity == RecentEvent && status.SemanticProgress == NoProgressEvidence {
			status.Health = HealthAliveNoProgress
		} else if status.StreamActivity == RecentEvent {
			status.Health = HealthAliveActive
		} else {
			status.Health = HealthAliveSilent
		}
	case processidentity.Dead, processidentity.Mismatch:
		status.ProcessLiveness = ProcessDead
		status.Health = HealthAbandoned
	case processidentity.Unknown:
		status.ProcessLiveness = ProcessUnknown
		status.Health = HealthUnverifiable
	}
	if state.stopRequested {
		status.Health = HealthAbandoned
	}
	return status, nil
}
