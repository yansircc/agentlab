package preparation

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

func (o *Operation) ProposeDecision(node DecisionNode) error {
	kind, err := nodeKind(node)
	if err != nil {
		return err
	}
	if (kind == HumanDecision || kind == BlockedExternalFact) && len(node.MaterialTo) == 0 {
		return errors.New("material decision boundary is required")
	}
	if kind == HumanDecision {
		refs := append([]artifact.Ref(nil), node.Human.Recommended.Evidence...)
		for _, option := range node.Human.Alternatives {
			refs = append(refs, option.Evidence...)
		}
		if err := o.requireArtifacts(refs); err != nil {
			return err
		}
	}
	return o.mutate(func(current *state) error {
		if current.source == nil {
			return ErrNotBegun
		}
		if current.sealed {
			return ErrSealed
		}
		if current.nodes[node.ID].ID != "" {
			return errors.New("decision node id already exists")
		}
		for _, dependency := range node.DependsOn {
			if current.nodes[dependency].ID == "" {
				return errors.New("decision dependencies must already exist")
			}
		}
		return o.append(eventNode, node)
	})
}

func (o *Operation) ResolveDecision(resolution Resolution) error {
	if resolution.NodeID == "" || resolution.Answer == "" || resolution.Authority == "" {
		return errors.New("node id, answer, and authority are required")
	}
	if err := o.requireArtifacts(resolution.Evidence); err != nil {
		return err
	}
	return o.mutate(func(current *state) error {
		if current.source == nil {
			return ErrNotBegun
		}
		if current.sealed {
			return ErrSealed
		}
		frontier := current.frontier()
		if frontier == nil || frontier.ID != resolution.NodeID {
			return ErrWrongFrontier
		}
		if err := validateResolution(*frontier, resolution); err != nil {
			return err
		}
		return o.append(eventResolution, resolution)
	})
}

func validateResolution(node DecisionNode, resolution Resolution) error {
	kind, err := nodeKind(node)
	if err != nil {
		return err
	}
	switch kind {
	case HumanDecision:
		if resolution.Authority != "human" || !humanOptionExists(*node.Human, resolution.OptionID) {
			return errors.New("human decision requires human authority and a declared option")
		}
	case DiscoverableFact:
		if resolution.Authority != "repository" || resolution.OptionID != "" || len(resolution.Evidence) == 0 {
			return errors.New("discoverable fact requires repository evidence")
		}
	case LowRiskAssumption:
		if resolution.Authority != "designer" || resolution.OptionID != "" {
			return errors.New("low-risk assumption requires designer authority")
		}
	case BlockedExternalFact:
		if resolution.Authority != "external" || resolution.OptionID != "" || len(resolution.Evidence) == 0 {
			return errors.New("external fact requires external evidence")
		}
	}
	return nil
}

func humanOptionExists(node HumanNode, id string) bool {
	if id == node.Recommended.ID {
		return true
	}
	for _, option := range node.Alternatives {
		if option.ID == id {
			return true
		}
	}
	return false
}
