package run

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/transaction"
)

type AttachedSpec struct {
	Adapter       string
	StreamID      string
	InitialCursor []byte
	Policy        StopPolicy
	Identity      *processidentity.Identity
	Capabilities  AdapterCapabilities
}

type AdapterCapabilities struct {
	DurableCursor     bool `json:"durable_cursor"`
	PublicEventsOnly  bool `json:"public_events_only"`
	ThinkingExclusion bool `json:"thinking_exclusion"`
	ToolCorrelation   bool `json:"tool_correlation"`
}

func RequiredAdapterCapabilities() AdapterCapabilities {
	return AdapterCapabilities{DurableCursor: true, PublicEventsOnly: true, ThinkingExclusion: true, ToolCorrelation: true}
}

type AdapterState struct {
	Adapter  string `json:"adapter"`
	StreamID string `json:"stream_id"`
	Cursor   []byte `json:"-"`
	Stopped  bool   `json:"stopped"`
}

type AdapterEvent struct {
	Kind          EvidenceKind
	CorrelationID string
	Raw           []byte
	Label         string
	CompactText   string
}

type AdapterExcluded struct {
	Category string
	Size     int
}

type AdapterBatch struct {
	Events     []AdapterEvent
	Exclusions []AdapterExcluded
}

type attachedRequestReceipt struct {
	Adapter      string              `json:"adapter"`
	StreamID     string              `json:"stream_id"`
	Policy       StopPolicy          `json:"policy"`
	Manifest     artifact.Ref        `json:"manifest"`
	Capabilities AdapterCapabilities `json:"capabilities"`
}

func (o *Operation) BeginAttached(spec AttachedSpec) (result AdapterState, resultErr error) {
	if err := validateAttachedSpec(spec); err != nil {
		return AdapterState{}, err
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return AdapterState{}, err
	}
	defer func() {
		if err := lease.Release(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	if err := o.requireUnstarted(); err != nil {
		return AdapterState{}, err
	}
	manifest, err := o.requireManifest()
	if err != nil {
		return AdapterState{}, err
	}
	cursor, err := o.artifacts.Put(spec.InitialCursor)
	if err != nil {
		return AdapterState{}, err
	}
	request := attachedRequestReceipt{Adapter: spec.Adapter, StreamID: spec.StreamID, Policy: spec.Policy, Manifest: manifest, Capabilities: spec.Capabilities}
	if err := o.writeReceipt("request.json", request); err != nil {
		return AdapterState{}, err
	}
	started := processStarted{
		Manifest: manifest,
		Process:  processHandle{Kind: processAttached, Identity: spec.Identity},
		Policy:   spec.Policy,
		Adapter:  &adapterBinding{Adapter: spec.Adapter, StreamID: spec.StreamID, Cursor: cursor, Capabilities: spec.Capabilities},
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventProcessStarted, started); err != nil {
		return AdapterState{}, err
	}
	return AdapterState{Adapter: spec.Adapter, StreamID: spec.StreamID, Cursor: append([]byte(nil), spec.InitialCursor...)}, nil
}

func validateAttachedSpec(spec AttachedSpec) error {
	if spec.Adapter == "" || spec.StreamID == "" || len(spec.InitialCursor) == 0 {
		return errors.New("adapter, stream id, and initial cursor are required")
	}
	if err := spec.Policy.Validate(); err != nil {
		return err
	}
	if spec.Policy.OwnsWorkerProcess || spec.Policy.KillOnHardIdle {
		return errors.New("attached runtime cannot grant process ownership")
	}
	if spec.Capabilities != RequiredAdapterCapabilities() {
		return errors.New("attached adapter capability downgrade is not allowed")
	}
	return nil
}

type AdapterWriter struct {
	operation *Operation
	lease     *transaction.Lease
	adapter   string
	streamID  string
	cursor    []byte
	closed    bool
}
