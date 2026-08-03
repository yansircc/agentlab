package transaction

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrValueExists = errors.New("path already contains a different value")

func WriteOnce(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return ErrValueExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".write-once-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); errors.Is(err, fs.ErrExist) {
		existing, readErr := os.ReadFile(path)
		if readErr == nil && bytes.Equal(existing, data) {
			return nil
		}
		return ErrValueExists
	} else if err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}
