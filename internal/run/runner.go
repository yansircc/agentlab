package run

import (
	"context"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/transaction"
)

type StartSpec struct {
	PublicCommand            []string          `json:"-"`
	PublicEnvironment        map[string]string `json:"-"`
	SecretEnvironmentHandles map[string]string `json:"-"`
	Policy                   StopPolicy        `json:"policy"`
}

type StartResult struct {
	RunID string `json:"run_id"`
	Code  int    `json:"exit_code"`
}

func (o *Operation) Start(ctx context.Context, runID string, spec StartSpec) (result StartResult, resultErr error) {
	if runID != o.runID {
		return StartResult{}, ErrInvalidRunID
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return StartResult{}, err
	}
	defer func() {
		if err := lease.Release(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	if err := o.requireUnstarted(); err != nil {
		return StartResult{}, err
	}
	manifest, err := o.requireManifest()
	if err != nil {
		return StartResult{}, err
	}
	return o.startOwned(ctx, runID, spec, manifest)
}
