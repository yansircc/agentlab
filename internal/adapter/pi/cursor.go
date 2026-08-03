package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

type header struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	ID      string `json:"id"`
}

func Attach(path string) (Cursor, error) {
	f, err := os.Open(path)
	if err != nil {
		return Cursor{}, err
	}
	defer f.Close()
	value, err := readHeader(f)
	if err != nil {
		return Cursor{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return Cursor{}, err
	}
	discard := false
	if info.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, info.Size()-1); err != nil {
			return Cursor{}, err
		}
		discard = last[0] != '\n'
	}
	return Cursor{SessionID: value.ID, Offset: info.Size(), DiscardUntilNewline: discard}, nil
}

func ReadNew(path string, cursor Cursor, sink Sink) (Cursor, error) {
	f, err := os.Open(path)
	if err != nil {
		return cursor, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return cursor, err
	}
	if info.Size() < cursor.Offset {
		return cursor, ErrSessionRewound
	}
	value, err := readHeader(f)
	if err != nil || value.ID != cursor.SessionID {
		return cursor, ErrInvalidSession
	}
	if _, err := f.Seek(cursor.Offset, io.SeekStart); err != nil {
		return cursor, err
	}
	return readTail(f, cursor, sink)
}

func readHeader(f *os.File) (header, error) {
	buffer := make([]byte, maxHeaderBytes+1)
	n, err := f.ReadAt(buffer, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return header{}, err
	}
	newline := bytes.IndexByte(buffer[:n], '\n')
	if newline < 0 || newline > maxHeaderBytes {
		return header{}, ErrInvalidSession
	}
	var value header
	if json.Unmarshal(buffer[:newline], &value) != nil || value.Type != "session" || value.Version != 3 || value.ID == "" {
		return header{}, ErrInvalidSession
	}
	return value, nil
}
