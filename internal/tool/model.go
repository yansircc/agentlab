// Package tool owns the four host-bound Supervisor tool operations.
package tool

import (
	"errors"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/preparation"
)

const (
	ApplyTool   = "agentlab_apply"
	RunTool     = "agentlab_run"
	InspectTool = "agentlab_inspect"
	CompareTool = "agentlab_compare"
)

// ActiveToolNames is the one closed owner of the Supervisor's provider-visible
// authority. The bundled extension and Host-side Pi launcher both derive their
// allowlist from it; neither keeps an independent copy.
func ActiveToolNames() []string {
	return []string{ApplyTool, RunTool, InspectTool, CompareTool}
}

// Binding is host authority, never model-authored input. Empty preparation or
// experiment bindings deny operations which require that scope.
type Binding struct {
	Root          string
	PreparationID string
	ExperimentID  string
	Authority     string
	Runtime       RuntimeHost
}

// RuntimeHost owns adapter locators, executable selection, session paths, and
// role capability profiles. The model can only select an opaque profile ref.
type RuntimeHost interface {
	StartIntent(Binding, StartRequest) (effect.Intent, error)
	CheckpointIntent(Binding, CheckpointRequest) (effect.Intent, error)
	ForkIntent(Binding, ForkRequest) (effect.Intent, error)
	Start(Binding, effect.Intent, string) (any, error)
	Poll(Binding, string, string) (any, error)
	Checkpoint(Binding, effect.Intent, string) (any, error)
	Fork(Binding, effect.Intent, string, string) (any, error)
}

type StartRequest struct {
	ID         string
	RunID      string
	RuntimeRef string
	Handoff    *artifact.Ref
}

type CheckpointRequest struct {
	ID           string
	RunID        string
	RuntimeRef   string
	EntryLocator string
}

type ForkRequest struct {
	ID         string
	RunID      string
	RuntimeRef string
	Checkpoint artifact.Ref
	ChildRun   string
}

func (b Binding) Validate() error {
	if b.Root == "" || !filepath.IsAbs(b.Root) {
		return errors.New("tool host root is invalid")
	}
	return nil
}

func (b Binding) store() artifact.Store {
	return artifact.NewStore(filepath.Join(b.Root, "artifacts"))
}

func (b Binding) authority() string {
	if b.Authority == "" {
		return "supervisor"
	}
	return b.Authority
}

func (b Binding) preparation() (*preparation.Operation, error) {
	if b.PreparationID == "" {
		return nil, errors.New("tool host has no preparation binding")
	}
	return preparation.Open(b.Root, b.PreparationID)
}

func (b Binding) experiment() (*experiment.Operation, error) {
	if b.ExperimentID == "" {
		return nil, errors.New("tool host has no experiment binding")
	}
	return experiment.Open(b.Root, b.ExperimentID)
}

// Operation is a closed decoded value. Only Decode can construct one from
// model bytes, keeping schema, stdin, and execution on one authority path.
type Operation interface {
	toolName() string
	execute(Binding) (any, error)
}

func Execute(binding Binding, value Operation) (any, error) {
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("tool operation is absent")
	}
	return value.execute(binding)
}
