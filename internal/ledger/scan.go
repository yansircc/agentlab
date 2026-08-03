package ledger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

func scan(path string, visit func(Record) error) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	reader := bufio.NewReaderSize(f, maxRecordBytes+1)
	var previousAt time.Time
	for expected := uint64(1); ; expected++ {
		line, err := reader.ReadSlice('\n')
		if errors.Is(err, io.EOF) {
			if len(line) != 0 {
				return ErrPartialFinal
			}
			return nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			return fmt.Errorf("%w at sequence %d", ErrCorrupt, expected)
		}
		if err != nil {
			return err
		}
		record, err := decodeRecord(line[:len(line)-1], expected, previousAt)
		if err != nil {
			return err
		}
		if err := visit(record); err != nil {
			return err
		}
		previousAt = record.At
	}
}

func decodeRecord(line []byte, expected uint64, previousAt time.Time) (Record, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("%w at sequence %d", ErrCorrupt, expected)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || record.Sequence != expected || record.Kind == "" || record.At.IsZero() || record.Data == nil || (!previousAt.IsZero() && record.At.Before(previousAt)) {
		return Record{}, fmt.Errorf("%w at sequence %d", ErrCorrupt, expected)
	}
	return record, nil
}
