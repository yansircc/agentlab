package preparation

import (
	"path/filepath"
	"regexp"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/transaction"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Operation struct {
	id        string
	dir       string
	ledger    *ledger.Ledger
	artifacts artifact.Store
}

func Open(root, id string) (*Operation, error) {
	if !idPattern.MatchString(id) {
		return nil, ErrInvalidID
	}
	dir := filepath.Join(root, "preparations", id)
	return &Operation{
		id: id, dir: dir,
		ledger:    ledger.Open(filepath.Join(dir, "events.jsonl")),
		artifacts: artifact.NewStore(filepath.Join(root, "artifacts")),
	}, nil
}

func (o *Operation) mutate(apply func(*state) error) (resultErr error) {
	lease, err := transaction.Acquire(filepath.Join(o.dir, "writer.lock"))
	if err != nil {
		return err
	}
	defer func() {
		if err := lease.Release(); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	current, err := o.current()
	if err != nil {
		return err
	}
	return apply(&current)
}

func (o *Operation) append(kind string, value any) error {
	_, err := o.ledger.Append(time.Now().UTC(), kind, value)
	return err
}

func (o *Operation) Inspect(after uint64, limit int) ([]ledger.Record, error) {
	return o.ledger.Read(after, limit)
}
