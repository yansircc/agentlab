package preparation

import "github.com/yansircc/agentlab/internal/artifact"

func (o *Operation) Status() (Status, error) {
	current, err := o.current()
	if err != nil {
		return Status{}, err
	}
	if current.input == nil {
		return Status{PreparationID: o.id, Phase: PhaseExploring}, nil
	}
	result := Status{
		PreparationID: o.id, WorkerInput: current.input.WorkerInput, EventCount: current.eventCount,
	}
	if current.source != nil {
		result.Source = current.source.SourceSnapshot
	}
	if current.assay != nil {
		copy := *current.assay
		copy.Evidence = append([]artifact.Ref(nil), current.assay.Evidence...)
		result.LeakageAssay = &copy
	}
	switch {
	case current.sealed:
		result.Phase = PhaseSealed
	case current.frontier() != nil:
		result.NextNode = current.frontier()
		kind, _ := nodeKind(*result.NextNode)
		if kind == HumanDecision {
			result.Phase = PhaseNeedsDecision
		} else if kind == BlockedExternalFact {
			result.Phase = PhaseBlocked
		} else {
			result.Phase = PhaseExploring
		}
	case current.assay == nil:
		result.Phase = PhaseExploring
	case current.assay.Verdict == LeakageDetected:
		result.Phase = PhaseBlocked
	case current.challenged && len(current.gaps) != 0:
		result.Phase, result.OpenGaps = PhaseChallenged, append([]ChallengeGap(nil), current.gaps...)
	case current.challenged:
		result.Phase = PhaseReady
	default:
		result.Phase = PhaseExploring
	}
	return result, nil
}
