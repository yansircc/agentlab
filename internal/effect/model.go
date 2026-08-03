// Package effect owns the closed identity shared by runtime intents and receipts.
package effect

import (
	"errors"
	"regexp"

	"github.com/yansircc/agentlab/internal/artifact"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Kind string

const (
	WorkerStart Kind = "worker_start"
	CoderStart  Kind = "coder_start"
	Stop        Kind = "stop"
	Checkpoint  Kind = "checkpoint"
	Fork        Kind = "fork"
)

type Intent struct {
	ID      string       `json:"id"`
	RunID   string       `json:"run_id"`
	Kind    Kind         `json:"kind"`
	Payload artifact.Ref `json:"payload"`
}

type Receipt struct {
	IntentID string       `json:"intent_id"`
	Kind     Kind         `json:"kind"`
	Evidence artifact.Ref `json:"evidence"`
}

func (value Kind) Validate() error {
	switch value {
	case WorkerStart, CoderStart, Stop, Checkpoint, Fork:
		return nil
	default:
		return errors.New("effect kind is invalid")
	}
}

func (value Intent) Validate() error {
	if !idPattern.MatchString(value.ID) || !idPattern.MatchString(value.RunID) || value.Kind.Validate() != nil || !validRef(value.Payload) {
		return errors.New("effect intent is invalid")
	}
	return nil
}

func (value Receipt) Validate() error {
	if !idPattern.MatchString(value.IntentID) || value.Kind.Validate() != nil || !validRef(value.Evidence) {
		return errors.New("effect receipt is invalid")
	}
	return nil
}

func validRef(ref artifact.Ref) bool {
	return ref.Algorithm == "sha256" && len(ref.Digest) == 64 && ref.Size >= 0
}
