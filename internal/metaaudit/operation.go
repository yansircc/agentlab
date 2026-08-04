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
	eventReview      = "meta_decision_reviewed"
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
	if state.findings[value.ID].ID != "" || state.assesses(value.DecisionID) || artifact.NewStore(filepath.Join(evaluatedRoot, "artifacts")).Scope() != state.trial.EvaluatedScope {
		return errors.New("meta-audit finding crosses capability roots")
	}
	if err := verifyFinding(evaluatedRoot, *state.trial, value); err != nil {
		return err
	}
	_, err = o.ledger.Append(time.Now().UTC(), eventFinding, value)
	return err
}

// RecordReview records a no-finding Codex assessment. Like a Finding, it is
// evaluated-root read-only and can cite no evidence only for the one sealed
// FreshOrigin bootstrap start decision.
func (o *Operation) RecordReview(evaluatedRoot string, value Review) error {
	state, err := o.current()
	if err != nil || state.trial == nil || state.sealed || value.Validate() != nil || value.GroundTruth != state.trial.GroundTruth {
		return errors.New("meta-audit review is invalid")
	}
	if state.reviews[value.ID].ID != "" || state.assesses(value.DecisionID) || artifact.NewStore(filepath.Join(evaluatedRoot, "artifacts")).Scope() != state.trial.EvaluatedScope {
		return errors.New("meta-audit review crosses capability roots")
	}
	if err := verifyReview(evaluatedRoot, *state.trial, value); err != nil {
		return err
	}
	_, err = o.ledger.Append(time.Now().UTC(), eventReview, value)
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
	if value.EvidenceThrough == 0 || len(value.WorkerEvidence) == 0 {
		return errors.New("meta-audit evaluated root is invalid")
	}
	if err := verifyAssessment(root, trial, value.DecisionID, value.WorkerRun, value.EvidenceThrough, value.WorkerEvidence); err != nil {
		return err
	}
	return verifyOracleEvidence(root, trial, value.WorkerRun, value.EvidenceThrough, value.OracleEvidence)
}

func verifyReview(root string, trial Trial, value Review) error {
	return verifyAssessment(root, trial, value.DecisionID, value.WorkerRun, value.EvidenceThrough, value.WorkerEvidence)
}

func verifyAssessment(root string, trial Trial, decisionID, workerRun string, evidenceThrough uint64, evidence []run.EvidenceRef) error {
	if !filepath.IsAbs(root) || workerRun == "" {
		return errors.New("meta-audit evaluated root is invalid")
	}
	experimentOp, err := experiment.Open(root, trial.ExperimentID)
	if err != nil {
		return err
	}
	decision, err := experimentOp.SupervisorDecision(decisionID)
	if err != nil || decision.WorkerRun != workerRun || decision.EvidenceThrough != evidenceThrough {
		return errors.New("meta-audit decision does not match evidence prefix")
	}
	if evidenceThrough == 0 {
		if decision.Action != experiment.DecisionWorkerStart || len(decision.Evidence) != 0 || len(evidence) != 0 {
			return errors.New("meta-audit bootstrap review is invalid")
		}
		return nil
	}
	for _, ref := range evidence {
		if ref.ExperimentID != trial.ExperimentID || ref.RunID != workerRun || ref.Sequence > evidenceThrough {
			return errors.New("meta-audit finding uses hindsight evidence")
		}
		runOp, err := run.Open(root, trial.ExperimentID, workerRun)
		if err != nil {
			return err
		}
		if _, err := runOp.EvidenceAt(ref); err != nil {
			return err
		}
	}
	return nil
}

func verifyOracleEvidence(root string, trial Trial, workerRun string, evidenceThrough uint64, evidence []run.EvidenceRef) error {
	if len(evidence) == 0 {
		return errors.New("meta-audit objective oracle evidence is absent")
	}
	runOp, err := run.Open(root, trial.ExperimentID, workerRun)
	if err != nil {
		return err
	}
	for _, ref := range evidence {
		if ref.ExperimentID != trial.ExperimentID || ref.RunID != workerRun || ref.Sequence > evidenceThrough {
			return errors.New("meta-audit oracle evidence uses another run or future evidence")
		}
		item, err := runOp.EvidenceAt(ref)
		if err != nil || item.Kind != run.EvidenceOracle {
			return errors.New("meta-audit objective oracle evidence is invalid")
		}
	}
	return nil
}

func decode(data []byte, value any) error { return strictjson.Decode(data, value) }
