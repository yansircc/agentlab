package ledger

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (l *Ledger) Append(at time.Time, kind string, value any) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var last uint64
	if err := scan(l.path, func(record Record) error {
		last = record.Sequence
		return nil
	}); err != nil {
		return Record{}, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return Record{}, err
	}
	record := Record{Sequence: last + 1, At: at.UTC(), Kind: kind, Data: data}
	line, err := json.Marshal(record)
	if err != nil {
		return Record{}, err
	}
	if len(line) > maxRecordBytes {
		return Record{}, ErrCorrupt
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return Record{}, err
	}
	_, statErr := os.Stat(l.path)
	created := errors.Is(statErr, os.ErrNotExist)
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Record{}, err
	}
	written, writeErr := f.Write(append(line, '\n'))
	if writeErr == nil && written != len(line)+1 {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		_ = f.Close()
		return Record{}, writeErr
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return Record{}, err
	}
	if err := f.Close(); err != nil {
		return Record{}, err
	}
	if created {
		if err := syncDirectory(filepath.Dir(l.path)); err != nil {
			return Record{}, err
		}
	}
	return record, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
