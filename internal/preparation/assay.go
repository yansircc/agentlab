package preparation

import "errors"

func (o *Operation) RecordLeakageAssay(value LeakageAssay) error {
	if !validLeakageAssay(value) {
		return errors.New("leakage assay is invalid")
	}
	if err := o.requireArtifacts(value.Evidence); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.source == nil {
			return ErrNotBegun
		}
		if current.sealed {
			return ErrSealed
		}
		if current.assay != nil {
			return errors.New("leakage assay already recorded")
		}
		if value.WorkerInput != current.input.WorkerInput || value.SourceSnapshot != current.source.SourceSnapshot {
			return errors.New("leakage assay does not bind exact preparation inputs")
		}
		if value.Reviewer == current.input.Authority {
			return errors.New("leakage assay reviewer is not independent")
		}
		return o.append(eventLeakageAssay, value)
	})
}
