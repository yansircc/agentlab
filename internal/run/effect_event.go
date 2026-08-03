package run

import (
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
)

const (
	eventRuntimeCheckpoint = "runtime_checkpoint_recorded"
	eventEffectReceipt     = "effect_receipt_recorded"
)

type runtimeCheckpointRecorded struct {
	Checkpoint   artifact.Ref `json:"checkpoint"`
	PublicPrefix artifact.Ref `json:"public_prefix"`
}

type effectReceiptRecorded struct {
	Receipt effect.Receipt `json:"receipt"`
}
