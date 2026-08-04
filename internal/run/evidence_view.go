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
	SourceLocator string       `json:"source_locator,omitempty"`
	Raw           artifact.Ref `json:"raw,omitempty"`
	CompactText   string       `json:"compact_text,omitempty"`
}

var ErrEvidenceSourceAbsent = errors.New("evidence source locator is absent")

// EvidenceForSourceLocator resolves the one durable admission for an opaque
// adapter-owned public source. A source may not be synthesized from a live
// session: absent and ambiguous mappings are both rejected.
func (o *Operation) EvidenceForSourceLocator(locator string) (EvidenceRef, error) {
	if !validSourceLocator(locator) {
		return EvidenceRef{}, errors.New("evidence source locator is invalid")
	}
	if _, err := o.currentState(); err != nil {
		return EvidenceRef{}, err
	}
	var result EvidenceRef
	err := o.ledger.Visit(func(record ledger.Record) error {
		items, err := projectEvidence(o.experimentID, o.runID, record)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.SourceLocator != locator {
				continue
			}
			if result != (EvidenceRef{}) {
				return errors.New("evidence source locator is ambiguous")
			}
			result = item.Ref
		}
		return nil
	})
	if err != nil {
		return EvidenceRef{}, err
	}
	if result == (EvidenceRef{}) {
		return EvidenceRef{}, ErrEvidenceSourceAbsent
	}
	return result, nil
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

// OracleEvidence returns every Host/adapter-admitted objective oracle item in
// the immutable run ledger. It does not infer an oracle result from Worker
// text, tool output, or a Supervisor claim.
func (o *Operation) OracleEvidence() ([]EvidenceItem, error) {
	records, err := o.ledger.Replay()
	if err != nil {
		return nil, err
	}
	if _, err := replayRun(records); err != nil {
		return nil, err
	}
	result := make([]EvidenceItem, 0)
	for _, record := range records {
		items, err := projectEvidence(o.experimentID, o.runID, record)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.Kind == EvidenceOracle {
				result = append(result, item)
			}
		}
	}
	return result, nil
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
	case eventHostOracle:
		var value hostOracleEvidence
		if json.Unmarshal(record.Data, &value) != nil {
			return nil, errors.New("invalid host oracle evidence record")
		}
		base.Kind, base.Label, base.Raw = EvidenceOracle, "host_objective_oracle", value.Raw
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
		item.Kind, item.Label, item.CorrelationID, item.SourceLocator = value.Kind, value.Label, value.CorrelationID, value.SourceLocator
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
