package preparation

import (
	"fmt"

	"github.com/yansircc/agentlab/internal/ledger"
)

type state struct {
	input       *inputSealed
	source      *sourceAttached
	facts       map[string]RepositoryFact
	nodes       map[string]DecisionNode
	order       []string
	resolutions map[string]Resolution
	challenged  bool
	gaps        []ChallengeGap
	assay       *LeakageAssay
	sealed      bool
	eventCount  uint64
}

func newState() state {
	return state{
		facts: map[string]RepositoryFact{}, nodes: map[string]DecisionNode{},
		resolutions: map[string]Resolution{},
	}
}

func (o *Operation) current() (state, error) {
	records, err := o.ledger.Replay()
	if err != nil {
		return state{}, err
	}
	return replay(records)
}

func replay(records []ledger.Record) (state, error) {
	current := newState()
	for _, record := range records {
		if err := current.apply(record); err != nil {
			return state{}, err
		}
		current.eventCount = record.Sequence
	}
	return current, nil
}

func (s *state) apply(record ledger.Record) error {
	if s.sealed {
		return fmt.Errorf("event after preparation seal at sequence %d", record.Sequence)
	}
	if record.Kind != eventWorkerInput && s.input == nil {
		return fmt.Errorf("preparation event before worker input seal at sequence %d", record.Sequence)
	}
	if record.Kind != eventWorkerInput && record.Kind != eventSource && s.source == nil {
		return fmt.Errorf("preparation event before source snapshot at sequence %d", record.Sequence)
	}
	switch record.Kind {
	case eventWorkerInput:
		var value inputSealed
		if record.Sequence != 1 || s.input != nil || decode(record.Data, &value) != nil || value.Authority == "" || !validRef(value.UserIntent) || !validRef(value.WorkerInput) {
			return invalid(record, "invalid worker input seal")
		}
		s.input = &value
	case eventSource:
		var value sourceAttached
		if record.Sequence != 2 || s.source != nil || decode(record.Data, &value) != nil || !validRef(value.SourceSnapshot) {
			return invalid(record, "invalid source snapshot attachment")
		}
		s.source = &value
	case eventFact:
		if s.source == nil {
			return invalid(record, "repository fact before source snapshot")
		}
		var value RepositoryFact
		if decode(record.Data, &value) != nil || !idPattern.MatchString(value.ID) || value.Statement == "" || len(value.Evidence) == 0 || s.facts[value.ID].ID != "" {
			return invalid(record, "invalid repository fact")
		}
		s.facts[value.ID] = value
		s.invalidateChallenge()
	case eventNode:
		if err := s.applyNode(record); err != nil {
			return err
		}
		s.invalidateChallenge()
	case eventResolution:
		if err := s.applyResolution(record); err != nil {
			return err
		}
		s.invalidateChallenge()
	case eventChallenge:
		var value Challenge
		if decode(record.Data, &value) != nil || !validRef(value.Basis) || !validGaps(value.Gaps) {
			return invalid(record, "invalid preparation challenge")
		}
		s.challenged, s.gaps = true, append([]ChallengeGap(nil), value.Gaps...)
	case eventLeakageAssay:
		var value LeakageAssay
		if decode(record.Data, &value) != nil || s.assay != nil || !validLeakageAssay(value) || value.WorkerInput != s.input.WorkerInput || value.SourceSnapshot != s.source.SourceSnapshot || value.Reviewer == s.input.Authority {
			return invalid(record, "invalid leakage assay")
		}
		s.assay = &value
		s.invalidateChallenge()
	case eventSealed:
		var value sealed
		if decode(record.Data, &value) != nil || s.source == nil || s.assay == nil || s.assay.Verdict != LeakageClean || value.WorkerInput != s.input.WorkerInput || s.frontier() != nil || !s.challenged || len(s.gaps) != 0 {
			return invalid(record, "invalid preparation seal")
		}
		s.sealed = true
	default:
		return fmt.Errorf("unknown preparation event %q", record.Kind)
	}
	return nil
}

func (s *state) invalidateChallenge() {
	s.challenged = false
	s.gaps = nil
}

func (s *state) applyNode(record ledger.Record) error {
	var node DecisionNode
	if decode(record.Data, &node) != nil || s.nodes[node.ID].ID != "" {
		return invalid(record, "invalid decision proposal")
	}
	if _, err := nodeKind(node); err != nil {
		return invalid(record, err.Error())
	}
	for _, dependency := range node.DependsOn {
		if dependency == node.ID || s.nodes[dependency].ID == "" {
			return invalid(record, "decision dependency is absent or cyclic")
		}
	}
	s.nodes[node.ID] = node
	s.order = append(s.order, node.ID)
	return nil
}

func (s *state) applyResolution(record ledger.Record) error {
	var value Resolution
	if decode(record.Data, &value) != nil || s.nodes[value.NodeID].ID == "" || s.resolutions[value.NodeID].NodeID != "" || value.Answer == "" || value.Authority == "" {
		return invalid(record, "invalid decision resolution")
	}
	frontier := s.frontier()
	if frontier == nil || frontier.ID != value.NodeID || validateResolution(*frontier, value) != nil {
		return invalid(record, "resolution does not match current frontier")
	}
	s.resolutions[value.NodeID] = value
	return nil
}

func (s *state) frontier() *DecisionNode {
	for _, id := range s.order {
		if s.resolutions[id].NodeID != "" {
			continue
		}
		node := s.nodes[id]
		ready := true
		for _, dependency := range node.DependsOn {
			ready = ready && s.resolutions[dependency].NodeID != ""
		}
		if ready {
			copy := node
			return &copy
		}
	}
	return nil
}

func invalid(record ledger.Record, reason string) error {
	return fmt.Errorf("%s at sequence %d", reason, record.Sequence)
}

func validGaps(gaps []ChallengeGap) bool {
	seen := map[string]bool{}
	for _, gap := range gaps {
		if !idPattern.MatchString(gap.ID) || gap.Statement == "" || seen[gap.ID] {
			return false
		}
		seen[gap.ID] = true
	}
	return true
}
