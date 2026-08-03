package metaaudit

import (
	"errors"
	"sort"

	"github.com/yansircc/agentlab/internal/ledger"
)

type state struct {
	trial      *Trial
	findings   map[string]Finding
	intervened bool
	sealed     bool
}

func (o *Operation) current() (state, error) {
	result := state{findings: map[string]Finding{}}
	err := o.ledger.Visit(func(record ledger.Record) error {
		if result.sealed {
			return errors.New("meta-audit event after seal")
		}
		switch record.Kind {
		case eventBegun:
			var value Trial
			if record.Sequence != 1 || result.trial != nil || decode(record.Data, &value) != nil || value.Validate() != nil {
				return errors.New("meta-audit begin event is invalid")
			}
			result.trial = &value
		case eventFinding:
			var value Finding
			if result.trial == nil || decode(record.Data, &value) != nil || value.Validate() != nil || value.GroundTruth != result.trial.GroundTruth || result.findings[value.ID].ID != "" {
				return errors.New("meta-audit finding event is invalid")
			}
			result.findings[value.ID] = value
		case eventIntervened:
			if result.trial == nil || result.intervened || len(record.Data) != 2 || string(record.Data) != "{}" {
				return errors.New("meta-audit intervention event is invalid")
			}
			result.intervened = true
		case eventTrialSealed:
			if result.trial == nil || len(record.Data) != 2 || string(record.Data) != "{}" {
				return errors.New("meta-audit seal event is invalid")
			}
			result.sealed = true
		default:
			return errors.New("unknown meta-audit event")
		}
		return nil
	})
	return result, err
}

type Status struct {
	Trial      Trial    `json:"trial"`
	FindingIDs []string `json:"finding_ids"`
	Intervened bool     `json:"intervened"`
	Sealed     bool     `json:"sealed"`
}

func (o *Operation) Status() (Status, error) {
	state, err := o.current()
	if err != nil || state.trial == nil {
		return Status{}, errors.New("meta-audit trial is absent")
	}
	result := Status{Trial: *state.trial, Intervened: state.intervened, Sealed: state.sealed}
	for id := range state.findings {
		result.FindingIDs = append(result.FindingIDs, id)
	}
	sort.Strings(result.FindingIDs)
	return result, nil
}
