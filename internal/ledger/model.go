package ledger

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

const maxRecordBytes = 8 * 1024 * 1024

var (
	ErrCorrupt      = errors.New("ledger corrupt")
	ErrPartialFinal = errors.New("ledger has partial final record")
)

type Record struct {
	Sequence uint64          `json:"sequence"`
	At       time.Time       `json:"at"`
	Kind     string          `json:"kind"`
	Data     json.RawMessage `json:"data"`
}

type Ledger struct {
	path string
	mu   sync.Mutex
}

func Open(path string) *Ledger { return &Ledger{path: path} }
