package experiment

import (
	"github.com/yansircc/agentlab/internal/preparation"
)

func (o *Operation) Begin(preparationID string) (Status, error) {
	prep, err := preparation.Open(o.root, preparationID)
	if err != nil {
		return Status{}, err
	}
	prepStatus, err := prep.Status()
	if err != nil {
		return Status{}, err
	}
	if prepStatus.Phase != preparation.PhaseSealed {
		return Status{}, ErrPreparationNotSealed
	}
	want := begun{PreparationID: preparationID, WorkerInput: prepStatus.WorkerInput, Source: prepStatus.Source}
	err = o.mutate(func(current *state) error {
		if current.begun == nil {
			return o.append(eventBegun, want)
		}
		if *current.begun != want {
			return ErrAlreadyBegun
		}
		return nil
	})
	if err != nil {
		return Status{}, err
	}
	return o.Status()
}
