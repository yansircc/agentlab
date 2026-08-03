package preparation

func (o *Operation) Seal() (Status, error) {
	err := o.mutate(func(current *state) error {
		if current.source == nil {
			return ErrNotBegun
		}
		if current.sealed {
			return ErrSealed
		}
		if current.frontier() != nil {
			return ErrUnresolved
		}
		if current.assay == nil {
			return ErrLeakageAssayRequired
		}
		if current.assay.Verdict != LeakageClean {
			return ErrLeakageDetected
		}
		if !current.challenged {
			return ErrChallengeNeeded
		}
		if len(current.gaps) != 0 {
			return ErrChallengeOpen
		}
		return o.append(eventSealed, sealed{WorkerInput: current.input.WorkerInput})
	})
	if err != nil {
		return Status{}, err
	}
	return o.Status()
}
