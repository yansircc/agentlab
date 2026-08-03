package pi

import "errors"

const (
	maxHeaderBytes = 1024 * 1024
	maxLineBytes   = 1024 * 1024
)

var (
	ErrInvalidSession = errors.New("invalid Pi session")
	ErrSessionRewound = errors.New("Pi session file shrank behind durable cursor")
	ErrLineTooLarge   = errors.New("Pi session record exceeds limit")
)

type Cursor struct {
	SessionID           string `json:"session_id"`
	Offset              int64  `json:"offset"`
	DiscardUntilNewline bool   `json:"discard_until_newline"`
}

type Event struct {
	Kind          string
	CorrelationID string
	Label         string
	CompactText   string
	Raw           []byte
}

type Exclusion struct {
	Category string
	Size     int
}

type Batch struct {
	Events     []Event
	Exclusions []Exclusion
}

type Sink interface {
	Commit(next Cursor, batch Batch) error
}
