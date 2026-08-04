package experiment

import (
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func (o *Operation) CommitDecisionBoundEffect(value DecisionBoundEffect) error {
	if value.Validate() != nil {
		return errors.New("decision-bound effect is invalid")
	}
	if _, err := o.artifacts.Read(value.Intent.Payload); err != nil {
		return err
	}
	if err := o.validateDecisionEvidence(value.Decision); err != nil {
		return err
	}
	if value.Decision.isBootstrapWorkerStart() {
		if err := o.requireUnstartedBootstrapWorker(value.Decision); err != nil {
			return err
		}
	}
	return o.mutate(func(current *state) error {
		if current.begun == nil {
			return ErrNotBegun
		}
		if current.runs[value.Decision.WorkerRun].RunID == "" || current.runs[value.Intent.RunID].RunID == "" {
			return errors.New("decision-bound effect targets an unbound run")
		}
		if _, exists := current.effects[value.Intent.ID]; exists {
			return errors.New("decision-bound effect id already exists")
		}
		if value.Decision.isBootstrapWorkerStart() && current.hasWorkerStart(value.Intent.RunID) {
			return errors.New("fresh Worker already has a start intent")
		}
		if value.Intent.Kind == effect.CoderStart {
			if _, err := o.coderStartProfile(*current, value); err != nil {
				return errors.New("coder start requires an experiment-owned handoff")
			}
		}
		return o.append(eventDecisionEffect, value)
	})
}

func (o *Operation) DecisionBoundEffect(id string) (DecisionBoundEffect, error) {
	current, err := o.current()
	if err != nil {
		return DecisionBoundEffect{}, err
	}
	value, exists := current.effects[id]
	if !exists {
		return DecisionBoundEffect{}, errors.New("decision-bound effect does not exist")
	}
	return value, nil
}

func (o *Operation) validateDecisionEvidence(value SupervisorDecision) error {
	if value.Validate() != nil {
		return errors.New("supervisor decision is invalid")
	}
	if value.isBootstrapWorkerStart() {
		return o.validateBootstrapWorkerStart(value)
	}
	for _, ref := range value.Evidence {
		if ref.ExperimentID != o.id || ref.RunID != value.WorkerRun || ref.Sequence > value.EvidenceThrough {
			return errors.New("supervisor decision evidence exceeds its public prefix")
		}
		operation, err := run.Open(o.root, o.id, value.WorkerRun)
		if err != nil {
			return err
		}
		if _, err := operation.EvidenceAt(ref); err != nil {
			return err
		}
	}
	return nil
}

func (o *Operation) validateBootstrapWorkerStart(value SupervisorDecision) error {
	current, err := o.replayUnvalidated()
	if err != nil || current.begun == nil {
		return errors.New("bootstrap Worker start is unavailable")
	}
	binding := current.runs[value.WorkerRun]
	if binding.RunID == "" {
		return errors.New("bootstrap Worker start targets an unbound run")
	}
	manifest, err := o.readManifest(binding.Manifest)
	if err != nil || !manifest.Origin.IsFresh() || manifest.WorkerInput != current.begun.WorkerInput {
		return errors.New("bootstrap Worker start does not target sealed fresh input")
	}
	return nil
}

// requireUnstartedBootstrapWorker checks the temporal condition only while
// committing the effect. Replay verifies the immutable sealed-input binding,
// but must remain valid after the Worker subsequently emits public evidence.
func (o *Operation) requireUnstartedBootstrapWorker(value SupervisorDecision) error {
	worker, err := run.Open(o.root, o.id, value.WorkerRun)
	if err != nil {
		return err
	}
	records, err := worker.Inspect(0, 1)
	if err != nil || len(records) != 0 {
		return errors.New("bootstrap Worker start requires an unstarted run")
	}
	return nil
}

func (o *Operation) SettleDecisionBoundEffect(id string, evidence []byte) (effect.Receipt, error) {
	value, err := o.DecisionBoundEffect(id)
	if err != nil {
		return effect.Receipt{}, err
	}
	operation, err := run.Open(o.root, o.id, value.Intent.RunID)
	if err != nil {
		return effect.Receipt{}, err
	}
	return operation.SettleEffect(value.Intent, evidence)
}
