package experiment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/yansircc/agentlab/internal/ledger"
)

func decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("event data has trailing input")
	}
	return nil
}

func invalid(record ledger.Record, reason string) error {
	return fmt.Errorf("%s at sequence %d", reason, record.Sequence)
}
