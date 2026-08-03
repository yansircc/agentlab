package metaaudit

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const (
	eventBegun       = "meta_trial_begun"
	eventFinding     = "meta_finding_recorded"
	eventIntervened  = "meta_intervened"
	eventTrialSealed = "meta_trial_sealed"
)

type Operation struct {
	root      string
	id        string
	ledger    *ledger.Ledger
	artifacts artifact.Store
}

func Open(root, id string) (*Operation, error) {
	if !filepath.IsAbs(root) || !idPattern.MatchString(id) {
		return nil, errors.New("meta-audit root or id is invalid")
	}
	return &Operation{root: root, id: id, ledger: ledger.Open(filepath.Join(root, "meta-audits", id, "events.jsonl")), artifacts: artifact.NewStore(filepath.Join(root, "artifacts"))}, nil
}

func (o *Operation) Begin(value Trial) error {
	if value.Validate() != nil || value.EvaluatedScope == o.artifacts.Scope() {
		return errors.New("meta-audit trial crosses capability roots")
	}
	if _, err := o.artifacts.Read(value.GroundTruth); err != nil {
		return err
	}
	state, err := o.current()
	if err != nil {
		return err
	}
	if state.trial != nil {
		if *state.trial == value {
			return nil
		}
		return errors.New("meta-audit trial already began")
	}
	_, err = o.ledger.Append(time.Now().UTC(), eventBegun, value)
	return err
}

func (o *Operation) Record(evaluatedRoot string, value Finding) error {
	state, err := o.current()
	if err != nil || state.trial == nil || state.sealed || value.Validate() != nil || value.GroundTruth != state.trial.GroundTruth {
		return errors.New("meta-audit finding is invalid")
	}
	if state.findings[value.ID].ID != "" || artifact.NewStore(filepath.Join(evaluatedRoot, "artifacts")).Scope() != state.trial.EvaluatedScope {
		return errors.New("meta-audit finding crosses capability roots")
	}
	if err := verifyFinding(evaluatedRoot, *state.trial, value); err != nil {
		return err
	}
	_, err = o.ledger.Append(time.Now().UTC(), eventFinding, value)
	return err
}

func (o *Operation) MarkIntervened() error {
	state, err := o.current()
	if err != nil || state.trial == nil || state.sealed || state.intervened {
		return errors.New("meta-audit intervention is invalid")
	}
	_, err = o.ledger.Append(time.Now().UTC(), eventIntervened, struct{}{})
	return err
}

func (o *Operation) Seal() error {
	state, err := o.current()
	if err != nil || state.trial == nil || state.sealed {
		return errors.New("meta-audit trial is not sealable")
	}
	_, err = o.ledger.Append(time.Now().UTC(), eventTrialSealed, struct{}{})
	return err
}

func verifyFinding(root string, trial Trial, value Finding) error {
	if !filepath.IsAbs(root) || value.WorkerRun == "" || value.EvidenceThrough == 0 {
		return errors.New("meta-audit evaluated root is invalid")
	}
	experimentOp, err := experiment.Open(root, trial.ExperimentID)
	if err != nil {
		return err
	}
	decision, err := experimentOp.SupervisorDecision(value.DecisionID)
	if err != nil || decision.WorkerRun != value.WorkerRun || decision.EvidenceThrough != value.EvidenceThrough {
		return errors.New("meta-audit decision does not match evidence prefix")
	}
	for _, ref := range value.WorkerEvidence {
		if ref.ExperimentID != trial.ExperimentID || ref.RunID != value.WorkerRun || ref.Sequence > value.EvidenceThrough {
			return errors.New("meta-audit finding uses hindsight evidence")
		}
		runOp, err := run.Open(root, trial.ExperimentID, value.WorkerRun)
		if err != nil {
			return err
		}
		if _, err := runOp.EvidenceAt(ref); err != nil {
			return err
		}
	}
	return nil
}

func decode(data []byte, value any) error { return strictjson.Decode(data, value) }
