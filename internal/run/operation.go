package run

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/transaction"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Operation struct {
	experimentID       string
	runID              string
	dir                string
	ledger             *ledger.Ledger
	artifacts          artifact.Store
	appendRun          func(time.Time, string, any) (ledger.Record, error)
	attemptProber      processidentity.Prober
	terminateIdentity  func(processidentity.Identity) error
	recordAttemptSpawn func(*launchAttempt, processidentity.Identity) error
}

func Open(root, experimentID, runID string) (*Operation, error) {
	if !runIDPattern.MatchString(experimentID) {
		return nil, ErrInvalidExperimentID
	}
	if !runIDPattern.MatchString(runID) {
		return nil, ErrInvalidRunID
	}
	dir := filepath.Join(root, "experiments", experimentID, "runs", runID)
	log := ledger.Open(filepath.Join(dir, "events.jsonl"))
	return &Operation{
		experimentID:      experimentID,
		runID:             runID,
		dir:               dir,
		ledger:            log,
		artifacts:         artifact.NewStore(filepath.Join(root, "artifacts")),
		appendRun:         log.Append,
		attemptProber:     processidentity.SystemProber{},
		terminateIdentity: terminateRecordedIdentity,
		recordAttemptSpawn: func(attempt *launchAttempt, identity processidentity.Identity) error {
			return attempt.recordSpawn(identity)
		},
	}, nil
}

func (o *Operation) Inspect(after uint64, limit int) ([]ledger.Record, error) {
	return o.ledger.Read(after, limit)
}

func (o *Operation) requireUnstarted() error {
	records, err := o.ledger.Read(0, 1)
	if err != nil {
		return err
	}
	if len(records) != 0 {
		return ErrRunStarted
	}
	return nil
}

func (o *Operation) writeReceipt(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return transaction.WriteOnce(filepath.Join(o.dir, name), data, 0o600)
}

func (o *Operation) appendEvent(at time.Time, kind string, value any) (ledger.Record, error) {
	return o.appendRun(at, kind, value)
}
