package preparation

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

// BeginRefs admits host-provided immutable task material. It is the provider
// path: a model selects already-issued refs and never supplies a filesystem
// locator or asks the kernel to import arbitrary bytes.
type BeginRefsSpec struct {
	UserIntent      artifact.Ref   `json:"user_intent"`
	SourceSnapshot  artifact.Ref   `json:"source_snapshot"`
	PublicArtifacts []artifact.Ref `json:"public_artifacts,omitempty"`
	Authority       string         `json:"authority"`
}

func (o *Operation) BeginRefs(spec BeginRefsSpec) (Status, error) {
	if spec.Authority == "" || !validRef(spec.UserIntent) || !validRef(spec.SourceSnapshot) {
		return Status{}, errors.New("preparation refs are invalid")
	}
	if _, err := o.artifacts.Read(spec.UserIntent); err != nil {
		return Status{}, err
	}
	if _, err := source.Load(o.artifacts, spec.SourceSnapshot); err != nil {
		return Status{}, err
	}
	for _, ref := range spec.PublicArtifacts {
		if !validRef(ref) {
			return Status{}, errors.New("public artifact ref is invalid")
		}
		if _, err := o.artifacts.Read(ref); err != nil {
			return Status{}, err
		}
	}
	workerInputValue := WorkerInput{Contract: workerInputContract, UserIntentRef: spec.UserIntent, PublicArtifacts: append([]artifact.Ref(nil), spec.PublicArtifacts...)}
	if !validWorkerInput(workerInputValue) {
		return Status{}, errors.New("worker input is invalid")
	}
	input, err := json.Marshal(workerInputValue)
	if err != nil {
		return Status{}, err
	}
	workerInput, err := o.artifacts.Put(input)
	if err != nil {
		return Status{}, err
	}
	wantInput := inputSealed{UserIntent: spec.UserIntent, WorkerInput: workerInput, Authority: spec.Authority}
	wantSource := sourceAttached{SourceSnapshot: spec.SourceSnapshot}
	err = o.mutate(func(current *state) error {
		if current.input == nil {
			if err := o.append(eventWorkerInput, wantInput); err != nil {
				return err
			}
			current.input = &wantInput
		} else if *current.input != wantInput {
			return ErrAlreadyBegun
		}
		if current.source == nil {
			return o.append(eventSource, wantSource)
		}
		if *current.source != wantSource {
			return ErrAlreadyBegun
		}
		return nil
	})
	if err != nil {
		return Status{}, err
	}
	return o.Status()
}
