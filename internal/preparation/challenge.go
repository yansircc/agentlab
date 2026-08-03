package preparation

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

func (o *Operation) Challenge(input Challenge) error {
	if !validRef(input.Basis) || !validGaps(input.Gaps) {
		return errors.New("challenge basis or gaps are invalid")
	}
	refs := []artifact.Ref{input.Basis}
	for _, gap := range input.Gaps {
		refs = append(refs, gap.Evidence...)
	}
	if err := o.requireArtifacts(refs); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.source == nil {
			return ErrNotBegun
		}
		if current.sealed {
			return ErrSealed
		}
		if current.frontier() != nil {
			return ErrUnresolved
		}
		expected, err := o.challengeBasis()
		if err != nil {
			return err
		}
		if input.Basis != expected {
			return errors.New("challenge basis does not match current preparation")
		}
		return o.append(eventChallenge, input)
	})
}

func (o *Operation) ChallengeBasis() (artifact.Ref, error) {
	current, err := o.current()
	if err != nil {
		return artifact.Ref{}, err
	}
	if current.source == nil {
		return artifact.Ref{}, ErrNotBegun
	}
	if current.sealed {
		return artifact.Ref{}, ErrSealed
	}
	return o.challengeBasis()
}

func (o *Operation) challengeBasis() (artifact.Ref, error) {
	records, err := o.ledger.Replay()
	if err != nil {
		return artifact.Ref{}, err
	}
	data, err := json.Marshal(records)
	if err != nil {
		return artifact.Ref{}, err
	}
	return o.artifacts.Put(data)
}
