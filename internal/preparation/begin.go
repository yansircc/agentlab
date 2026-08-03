package preparation

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
)

func (o *Operation) Begin(spec BeginSpec) (Status, error) {
	if len(spec.UserIntent) == 0 || len(spec.SourceFiles) == 0 || spec.Authority == "" {
		return Status{}, errors.New("user intent, source snapshot, and authority are required")
	}
	intent, err := o.artifacts.Put(spec.UserIntent)
	if err != nil {
		return Status{}, err
	}
	sourceRef, err := source.Build(o.artifacts, spec.SourceFiles)
	if err != nil {
		return Status{}, err
	}
	public := make([]artifact.Ref, 0, len(spec.PublicArtifacts))
	for _, value := range spec.PublicArtifacts {
		ref, err := o.artifacts.Put(value)
		if err != nil {
			return Status{}, err
		}
		public = append(public, ref)
	}
	inputBytes, err := json.Marshal(WorkerInput{Contract: workerInputContract, UserIntentRef: intent, PublicArtifacts: public})
	if err != nil {
		return Status{}, err
	}
	workerInput, err := o.artifacts.Put(inputBytes)
	if err != nil {
		return Status{}, err
	}
	wantInput := inputSealed{UserIntent: intent, WorkerInput: workerInput, Authority: spec.Authority}
	wantSource := sourceAttached{SourceSnapshot: sourceRef}
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
