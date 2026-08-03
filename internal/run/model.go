package run

import (
	"encoding/json"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/processidentity"
)

const (
	eventProcessStarted   = "process_started"
	eventEvidence         = "evidence"
	eventProgressObserved = "progress_observed"
	eventNoProgress       = "no_progress_evidence"
	eventStreamClosed     = "stream_closed"
	eventStreamCorrupt    = "stream_corrupt"
	eventFirstTimeout     = "first_event_timeout"
	eventSoftIdle         = "soft_idle"
	eventHardIdle         = "hard_idle"
	eventStopRequested    = "stop_requested"
	eventProcessExited    = "process_exited"
	eventTerminalAccepted = "terminal_accepted"
	eventTerminalRejected = "terminal_rejected"
	eventAdapterBatch     = "adapter_batch"
)

type StopPolicy struct {
	FirstEventTimeout time.Duration `json:"first_event_timeout"`
	SoftIdleTimeout   time.Duration `json:"soft_idle_timeout"`
	HardIdleTimeout   time.Duration `json:"hard_idle_timeout"`
	OwnsWorkerProcess bool          `json:"owns_worker_process"`
	KillOnHardIdle    bool          `json:"kill_on_hard_idle"`
}

func (p StopPolicy) Validate() error {
	if p.FirstEventTimeout <= 0 || p.SoftIdleTimeout <= 0 || p.HardIdleTimeout <= 0 {
		return ErrInvalidPolicy
	}
	if p.SoftIdleTimeout >= p.HardIdleTimeout {
		return ErrInvalidPolicy
	}
	if p.KillOnHardIdle && !p.OwnsWorkerProcess {
		return ErrInvalidPolicy
	}
	return nil
}

type ProcessLiveness string

const (
	ProcessAlive   ProcessLiveness = "alive"
	ProcessDead    ProcessLiveness = "dead"
	ProcessUnknown ProcessLiveness = "unknown"
)

type StreamActivity string

const (
	NoEventYet    StreamActivity = "no_event_yet"
	RecentEvent   StreamActivity = "recent_event"
	SoftIdle      StreamActivity = "soft_idle"
	HardIdle      StreamActivity = "hard_idle"
	StreamClosed  StreamActivity = "stream_closed"
	StreamCorrupt StreamActivity = "stream_corrupt"
)

type SemanticProgress string

const (
	ProgressObserved   SemanticProgress = "observed"
	NoProgressEvidence SemanticProgress = "not_observed"
	ProgressUnknown    SemanticProgress = "unknown"
)

type Health string

const (
	HealthStarting        Health = "starting"
	HealthAliveActive     Health = "alive_active"
	HealthAliveSilent     Health = "alive_silent"
	HealthAliveNoProgress Health = "alive_no_progress"
	HealthExitedClean     Health = "exited_clean"
	HealthExitedError     Health = "exited_error"
	HealthAbandoned       Health = "abandoned"
	HealthUnverifiable    Health = "unverifiable"
	HealthTerminalCorrupt Health = "terminal_corrupt"
)

type Status struct {
	Health             Health                    `json:"health"`
	ProcessLiveness    ProcessLiveness           `json:"process_liveness"`
	StreamActivity     StreamActivity            `json:"stream_activity"`
	SemanticProgress   SemanticProgress          `json:"semantic_progress"`
	FirstEventTimedOut bool                      `json:"first_event_timed_out"`
	StopRequested      bool                      `json:"stop_requested"`
	EventCount         uint64                    `json:"event_count"`
	LastEventAt        *time.Time                `json:"last_event_at,omitempty"`
	ProcessIdentity    *processidentity.Identity `json:"process_identity,omitempty"`
	Adapter            *AdapterIdentity          `json:"adapter,omitempty"`
	Deadlines          DeadlineProjection        `json:"deadlines"`
}

type AdapterIdentity struct {
	Name     string `json:"name"`
	StreamID string `json:"stream_id"`
}

type DeadlineProjection struct {
	FirstEvent *time.Time `json:"first_event,omitempty"`
	SoftIdle   *time.Time `json:"soft_idle,omitempty"`
	HardIdle   *time.Time `json:"hard_idle,omitempty"`
}

type processKind string

const (
	processOwned    processKind = "owned"
	processAttached processKind = "attached"
	processManaged  processKind = "managed_adapter"
)

type processHandle struct {
	Kind     processKind               `json:"kind"`
	Identity *processidentity.Identity `json:"identity,omitempty"`
}

type adapterBinding struct {
	Adapter      string              `json:"adapter"`
	StreamID     string              `json:"stream_id"`
	Cursor       artifact.Ref        `json:"cursor"`
	Capabilities AdapterCapabilities `json:"capabilities"`
}

type processStarted struct {
	AttemptID string          `json:"attempt_id,omitempty"`
	Manifest  artifact.Ref    `json:"manifest"`
	Process   processHandle   `json:"process"`
	Policy    StopPolicy      `json:"policy"`
	Adapter   *adapterBinding `json:"adapter,omitempty"`
}

type evidence struct {
	Stream string       `json:"stream"`
	Raw    artifact.Ref `json:"raw"`
	Label  string       `json:"label"`
}

type streamFact struct {
	Stream string `json:"stream"`
	Error  string `json:"error,omitempty"`
}

type progressFact struct {
	Detector string `json:"detector"`
}

type processExited struct {
	Code int `json:"code"`
}

type terminalResult struct {
	Contract string          `json:"contract"`
	Outcome  string          `json:"outcome"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type terminalRejected struct {
	Reason string `json:"reason"`
}

type stopEvent struct {
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
}

type adapterAdmission struct {
	Kind          EvidenceKind `json:"kind"`
	CorrelationID string       `json:"correlation_id,omitempty"`
	Raw           artifact.Ref `json:"raw"`
	Label         string       `json:"label"`
	CompactText   string       `json:"compact_text,omitempty"`
}

type adapterExclusion struct {
	Category string `json:"category"`
	Size     int    `json:"size"`
}

type adapterBatch struct {
	Adapter    string             `json:"adapter"`
	StreamID   string             `json:"stream_id"`
	Cursor     artifact.Ref       `json:"cursor"`
	Admissions []adapterAdmission `json:"admissions"`
	Exclusions []adapterExclusion `json:"exclusions"`
}
