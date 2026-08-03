package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
)

type EvidenceRef struct {
	ExperimentID string `json:"experiment_id"`
	RunID        string `json:"run_id"`
	Sequence     uint64 `json:"sequence"`
	Item         int    `json:"item"`
}

type EvidenceItem struct {
	Ref           EvidenceRef  `json:"ref"`
	At            time.Time    `json:"at"`
	Kind          EvidenceKind `json:"kind"`
	Label         string       `json:"label"`
	CorrelationID string       `json:"correlation_id,omitempty"`
	Raw           artifact.Ref `json:"raw,omitempty"`
	CompactText   string       `json:"compact_text,omitempty"`
}

func (o *Operation) EvidenceAt(ref EvidenceRef) (EvidenceItem, error) {
	if ref.ExperimentID != o.experimentID || ref.RunID != o.runID || ref.Sequence == 0 || ref.Item < 0 {
		return EvidenceItem{}, errors.New("evidence reference targets a different run or invalid position")
	}
	state := initialReplayState()
	var items []EvidenceItem
	err := o.ledger.Visit(func(record ledger.Record) error {
		if err := state.apply(record); err != nil {
			return err
		}
		if record.Sequence == ref.Sequence {
			var err error
			items, err = projectEvidence(o.experimentID, o.runID, record)
			return err
		}
		return nil
	})
	if err != nil {
		return EvidenceItem{}, err
	}
	if len(items) == 0 {
		return EvidenceItem{}, errors.New("evidence record does not exist")
	}
	if ref.Item >= len(items) {
		return EvidenceItem{}, errors.New("evidence item does not exist")
	}
	item := items[ref.Item]
	if item.Raw.Digest != "" {
		if _, err := o.artifacts.Read(item.Raw); err != nil {
			return EvidenceItem{}, fmt.Errorf("evidence artifact unavailable: %w", err)
		}
	}
	return item, nil
}

func projectEvidence(experimentID, runID string, record ledger.Record) ([]EvidenceItem, error) {
	base := EvidenceItem{Ref: EvidenceRef{ExperimentID: experimentID, RunID: runID, Sequence: record.Sequence}, At: record.At, Kind: EvidenceProcess, Label: record.Kind}
	switch record.Kind {
	case eventEvidence:
		var value evidence
		if json.Unmarshal(record.Data, &value) != nil {
			return nil, errors.New("invalid evidence record")
		}
		base.Label, base.Raw = value.Label, value.Raw
		return []EvidenceItem{base}, nil
	case eventAdapterBatch:
		return projectAdapterEvidence(base, record.Data)
	default:
		return []EvidenceItem{base}, nil
	}
}

func projectAdapterEvidence(base EvidenceItem, data []byte) ([]EvidenceItem, error) {
	var batch adapterBatch
	if json.Unmarshal(data, &batch) != nil {
		return nil, errors.New("invalid adapter evidence record")
	}
	items := make([]EvidenceItem, 0, len(batch.Admissions)+len(batch.Exclusions))
	for _, value := range batch.Admissions {
		item := base
		item.Ref.Item = len(items)
		item.Kind, item.Label, item.CorrelationID = value.Kind, value.Label, value.CorrelationID
		item.Raw, item.CompactText = value.Raw, value.CompactText
		items = append(items, item)
	}
	for _, value := range batch.Exclusions {
		item := base
		item.Ref.Item = len(items)
		item.Kind, item.Label = EvidenceExcluded, value.Category
		item.CompactText = fmt.Sprintf("%d bytes excluded", value.Size)
		items = append(items, item)
	}
	if len(items) == 0 {
		return []EvidenceItem{base}, nil
	}
	return items, nil
}
