package preparation

import "errors"

func (o *Operation) RecordFact(fact RepositoryFact) error {
	if !idPattern.MatchString(fact.ID) || fact.Statement == "" || len(fact.Evidence) == 0 {
		return errors.New("repository fact id, statement, and evidence are required")
	}
	if err := o.requireArtifacts(fact.Evidence); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.source == nil {
			return ErrNotBegun
		}
		if current.sealed {
			return ErrSealed
		}
		if current.facts[fact.ID].ID != "" {
			return errors.New("repository fact id already exists")
		}
		return o.append(eventFact, fact)
	})
}
