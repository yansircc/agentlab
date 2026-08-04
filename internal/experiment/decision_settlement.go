package experiment

import (
	"errors"
	"sort"
	"time"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/run"
)

type ReceiptStatus struct {
	RunID   string         `json:"run_id"`
	Receipt effect.Receipt `json:"receipt"`
}

type EffectSettlement struct {
	Pending    []effect.Intent `json:"pending"`
	Orphan     []ReceiptStatus `json:"orphan_receipts"`
	Mismatched []ReceiptStatus `json:"mismatched_receipts"`
}

func (o *Operation) EffectSettlement() (EffectSettlement, error) {
	current, err := o.current()
	if err != nil {
		return EffectSettlement{}, err
	}
	settled := map[string]bool{}
	result := EffectSettlement{}
	for _, runID := range current.runOrder {
		operation, err := run.Open(o.root, o.id, runID)
		if err != nil {
			return EffectSettlement{}, err
		}
		receipts, err := operation.EffectReceipts()
		if err != nil {
			return EffectSettlement{}, err
		}
		for _, receipt := range receipts {
			status := ReceiptStatus{RunID: runID, Receipt: receipt}
			intent, exists := current.effects[receipt.IntentID]
			switch {
			case !exists:
				result.Orphan = append(result.Orphan, status)
			case intent.Intent.RunID != runID || intent.Intent.Kind != receipt.Kind:
				result.Mismatched = append(result.Mismatched, status)
			default:
				settled[receipt.IntentID] = true
			}
		}
	}
	for id, value := range current.effects {
		if !settled[id] {
			result.Pending = append(result.Pending, value.Intent)
		}
	}
	sort.Slice(result.Pending, func(i, j int) bool { return result.Pending[i].ID < result.Pending[j].ID })
	sort.Slice(result.Orphan, func(i, j int) bool { return result.Orphan[i].Receipt.IntentID < result.Orphan[j].Receipt.IntentID })
	sort.Slice(result.Mismatched, func(i, j int) bool {
		return result.Mismatched[i].Receipt.IntentID < result.Mismatched[j].Receipt.IntentID
	})
	return result, nil
}

func (o *Operation) requireSettledEffects() error {
	settlement, err := o.EffectSettlement()
	if err != nil {
		return err
	}
	if len(settlement.Pending) != 0 || len(settlement.Orphan) != 0 || len(settlement.Mismatched) != 0 {
		return errors.New("decision-bound effects are not settled")
	}
	return nil
}

// RequireSettledStartEffects is the final-gate check for runtime entry. A
// bound run that has actually started must have exactly one decision-bound
// WorkerStart or CoderStart effect whose decision does not follow the durable
// process-start record and whose receipt verifies the observed start. It is
// deliberately separate from generic gate
// recording so Host-only fixture construction does not masquerade as a
// Supervisor-controlled recursive trial.
func (o *Operation) RequireSettledStartEffects() error {
	current, err := o.current()
	if err != nil {
		return err
	}
	times, err := o.startDecisionTimes()
	if err != nil {
		return err
	}
	for _, runID := range current.runOrder {
		operation, err := run.Open(o.root, o.id, runID)
		if err != nil {
			return err
		}
		records, err := operation.Inspect(0, 1)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			continue
		}
		if records[0].Kind != "process_started" {
			return errors.New("run start record is invalid")
		}
		var start *DecisionBoundEffect
		for _, id := range current.effectOrder {
			value := current.effects[id]
			if value.Intent.RunID != runID || (value.Intent.Kind != effect.WorkerStart && value.Intent.Kind != effect.CoderStart) {
				continue
			}
			if start != nil {
				return errors.New("run has multiple decision-bound start effects")
			}
			copy := value
			start = &copy
		}
		if start == nil {
			return errors.New("run start has no decision-bound effect")
		}
		decisionAt, exists := times[start.Intent.ID]
		if !exists || decisionAt.After(records[0].At) {
			return errors.New("run start precedes its decision-bound effect")
		}
		if err := operation.VerifyStartEffect(start.Intent); err != nil {
			return errors.New("run start effect is not settled")
		}
	}
	return nil
}

// RequireVerifiedRuntimeEffects is the recursive-gate proof for every
// decision-bound runtime effect. A generic receipt only proves that bytes were
// recorded; this method also proves the corresponding durable runtime fact and
// rejects a decision appended after that fact occurred.
func (o *Operation) RequireVerifiedRuntimeEffects() error {
	current, err := o.current()
	if err != nil {
		return err
	}
	if err := o.requireSettledEffects(); err != nil {
		return err
	}
	for _, id := range current.effectOrder {
		bound := current.effects[id]
		decisionAt, err := o.decisionBoundEffectTime(id)
		if err != nil {
			return err
		}
		operation, err := run.Open(o.root, o.id, bound.Intent.RunID)
		if err != nil {
			return err
		}
		switch bound.Intent.Kind {
		case effect.WorkerStart, effect.CoderStart:
			records, err := operation.Inspect(0, 1)
			if err != nil || len(records) != 1 || records[0].Kind != "process_started" || decisionAt.After(records[0].At) {
				return errors.New("runtime start precedes its decision-bound effect")
			}
			if err := operation.VerifyStartEffect(bound.Intent); err != nil {
				return errors.New("runtime start effect is not verified")
			}
		case effect.Stop:
			result, err := operation.VerifyStopEffect(bound.Intent)
			if err != nil || decisionAt.After(result.At) {
				return errors.New("runtime stop effect is not verified")
			}
		case effect.Checkpoint:
			checkpoint, err := operation.VerifyCheckpointEffect(bound.Intent)
			checkpointAt, checkpointErr := operation.RuntimeCheckpointTime(checkpoint.Checkpoint)
			if err != nil || checkpointErr != nil || decisionAt.After(checkpointAt) {
				return errors.New("runtime checkpoint effect is not verified")
			}
		case effect.Fork:
			forked, err := operation.VerifyForkEffect(bound.Intent)
			forkAt, forkErr := operation.SessionForkedTime(forked.ChildSession)
			if err != nil || forkErr != nil || decisionAt.After(forkAt) {
				return errors.New("runtime fork effect is not verified")
			}
		default:
			return errors.New("unknown decision-bound runtime effect")
		}
	}
	return nil
}

func (o *Operation) startDecisionTimes() (map[string]time.Time, error) {
	result := map[string]time.Time{}
	err := o.ledger.Visit(func(record ledger.Record) error {
		if record.Kind != eventDecisionEffect {
			return nil
		}
		var value DecisionBoundEffect
		if err := decode(record.Data, &value); err != nil {
			return err
		}
		if value.Intent.Kind == effect.WorkerStart || value.Intent.Kind == effect.CoderStart {
			result[value.Intent.ID] = record.At
		}
		return nil
	})
	return result, err
}
