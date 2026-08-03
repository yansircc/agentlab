package preparation

import "github.com/yansircc/agentlab/internal/artifact"

const (
	eventWorkerInput  = "worker_input_sealed"
	eventSource       = "source_snapshot_attached"
	eventFact         = "repository_fact"
	eventNode         = "decision_proposed"
	eventResolution   = "decision_resolved"
	eventChallenge    = "preparation_challenged"
	eventLeakageAssay = "leakage_assay_recorded"
	eventSealed       = "preparation_sealed"
)

type inputSealed struct {
	UserIntent  artifact.Ref `json:"user_intent"`
	WorkerInput artifact.Ref `json:"worker_input"`
	Authority   string       `json:"authority"`
}

type sourceAttached struct {
	SourceSnapshot artifact.Ref `json:"source_snapshot"`
}

type sealed struct {
	WorkerInput artifact.Ref `json:"worker_input"`
}
