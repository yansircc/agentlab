package preparation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/yansircc/agentlab/internal/artifact"
)

func decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("event data has trailing input")
	}
	return nil
}

func nodeKind(node DecisionNode) (DecisionKind, error) {
	count := 0
	var kind DecisionKind
	if node.Human != nil {
		count++
		kind = HumanDecision
	}
	if node.Fact != nil {
		count++
		kind = DiscoverableFact
	}
	if node.Assumption != nil {
		count++
		kind = LowRiskAssumption
	}
	if node.ExternalFact != nil {
		count++
		kind = BlockedExternalFact
	}
	if count != 1 {
		return "", errors.New("decision node must contain exactly one variant")
	}
	return kind, validateNodeVariant(node, kind)
}

func validateNodeVariant(node DecisionNode, kind DecisionKind) error {
	if !idPattern.MatchString(node.ID) {
		return ErrInvalidID
	}
	if (kind == HumanDecision || kind == BlockedExternalFact) && len(node.MaterialTo) == 0 {
		return errors.New("material decision boundary is required")
	}
	switch kind {
	case DiscoverableFact:
		if node.Fact.Query == "" {
			return errors.New("discoverable fact query is required")
		}
	case HumanDecision:
		return validateHuman(*node.Human)
	case LowRiskAssumption:
		if node.Assumption.Statement == "" || node.Assumption.Consequence == "" {
			return errors.New("assumption statement and consequence are required")
		}
	case BlockedExternalFact:
		if node.ExternalFact.Requirement == "" {
			return errors.New("external fact requirement is required")
		}
	}
	return nil
}

func validateHuman(node HumanNode) error {
	if node.Question == "" || len(node.Alternatives) == 0 || len(node.Recommended.Evidence) == 0 {
		return errors.New("human decision requires a question, evidence-backed recommendation, and alternatives")
	}
	seen := map[string]bool{}
	for _, option := range append([]DecisionOption{node.Recommended}, node.Alternatives...) {
		if !idPattern.MatchString(option.ID) || option.Label == "" || option.Consequences == "" || seen[option.ID] {
			return errors.New("human decision option is invalid or duplicated")
		}
		for _, ref := range option.Evidence {
			if !validRef(ref) {
				return errors.New("human decision evidence reference is invalid")
			}
		}
		seen[option.ID] = true
	}
	return nil
}

func validRef(ref artifact.Ref) bool {
	return ref.Valid()
}

func (o *Operation) requireArtifacts(refs []artifact.Ref) error {
	for _, ref := range refs {
		if !validRef(ref) {
			return errors.New("invalid artifact reference")
		}
		if _, err := o.artifacts.Read(ref); err != nil {
			return fmt.Errorf("artifact evidence unavailable: %w", err)
		}
	}
	return nil
}

func validLeakageAssay(value LeakageAssay) bool {
	if !validRef(value.WorkerInput) || !validRef(value.SourceSnapshot) || value.Reviewer == "" || value.Authority != "reviewer" || value.Method == "" || len(value.Evidence) == 0 || len(value.Evidence) > 100 {
		return false
	}
	if value.Verdict != LeakageClean && value.Verdict != LeakageDetected {
		return false
	}
	for _, ref := range value.Evidence {
		if !validRef(ref) {
			return false
		}
	}
	return true
}
