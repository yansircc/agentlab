package run

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/transaction"
)

const operationStatusContract = "agentlab.operation-status.v1"

type OperationStatusProjection struct {
	Contract string    `json:"contract"`
	AsOf     time.Time `json:"as_of"`
	Status   Status    `json:"status"`
}

func (o *Operation) ProjectStatus(prober processidentity.Prober) (OperationStatusProjection, error) {
	return o.ProjectStatusAt(prober, time.Now().UTC())
}

func (o *Operation) ProjectStatusAt(prober processidentity.Prober, now time.Time) (OperationStatusProjection, error) {
	status, err := o.StatusAt(prober, now)
	if err != nil {
		return OperationStatusProjection{}, err
	}
	projection := OperationStatusProjection{Contract: operationStatusContract, AsOf: now.UTC(), Status: status}
	data, err := json.Marshal(projection)
	if err != nil {
		return OperationStatusProjection{}, err
	}
	if err := transaction.Replace(filepath.Join(o.dir, "result.json"), data, 0o600); err != nil {
		return OperationStatusProjection{}, err
	}
	if err := o.projectProcessReceipt(); err != nil {
		return OperationStatusProjection{}, err
	}
	return projection, nil
}

func (o *Operation) projectProcessReceipt() error {
	state, err := o.currentState()
	if err != nil || state.started == nil || state.started.Process.Identity == nil {
		return err
	}
	data, err := json.Marshal(state.started.Process.Identity)
	if err != nil {
		return err
	}
	return transaction.Replace(filepath.Join(o.dir, "process.json"), data, 0o600)
}
