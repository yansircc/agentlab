package run

import "time"

func (s replayState) projectActivity(now time.Time, eventCount uint64) Status {
	status := Status{
		Health:             HealthStarting,
		ProcessLiveness:    ProcessUnknown,
		StreamActivity:     NoEventYet,
		SemanticProgress:   s.semanticProgress,
		FirstEventTimedOut: s.firstEventTimedOut,
		StopRequested:      s.stopRequested,
		EventCount:         eventCount,
		LastEventAt:        s.lastEventAt,
	}
	if s.started == nil {
		return status
	}
	if s.started.Process.Identity != nil {
		identity := *s.started.Process.Identity
		status.ProcessIdentity = &identity
		deadline := s.startedAt.Add(s.started.Policy.FirstEventTimeout)
		status.Deadlines.FirstEvent = &deadline
	}
	if s.started.Adapter != nil {
		status.Adapter = &AdapterIdentity{Name: s.started.Adapter.Adapter, StreamID: s.started.Adapter.StreamID}
	}
	if s.streamCorrupt {
		status.StreamActivity = StreamCorrupt
		return status
	}
	if s.closedStreams["stdout"] && s.closedStreams["stderr"] {
		status.StreamActivity = StreamClosed
		return status
	}
	activityAt := *s.startedAt
	if s.lastEvidenceAt != nil {
		activityAt = *s.lastEvidenceAt
	} else if s.started.Process.Identity != nil && now.Sub(*s.startedAt) >= s.started.Policy.FirstEventTimeout {
		status.FirstEventTimedOut = true
	}
	softDeadline := activityAt.Add(s.started.Policy.SoftIdleTimeout)
	hardDeadline := activityAt.Add(s.started.Policy.HardIdleTimeout)
	status.Deadlines.SoftIdle, status.Deadlines.HardIdle = &softDeadline, &hardDeadline
	idleFor := now.Sub(activityAt)
	switch {
	case idleFor >= s.started.Policy.HardIdleTimeout:
		status.StreamActivity = HardIdle
	case idleFor >= s.started.Policy.SoftIdleTimeout:
		status.StreamActivity = SoftIdle
	case s.lastEvidenceAt != nil:
		status.StreamActivity = RecentEvent
	default:
		status.StreamActivity = NoEventYet
	}
	return status
}
