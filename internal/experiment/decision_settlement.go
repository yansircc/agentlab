package experiment

import (
	"errors"
	"sort"

	"github.com/yansircc/agentlab/internal/effect"
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
