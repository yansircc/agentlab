package pi

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type sessionParentHeader struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	ID            string `json:"id"`
	ParentSession string `json:"parentSession"`
}

func reconcileFork(attempt forkAttempt) (string, error) {
	entries, err := os.ReadDir(attempt.ChildSessionDir)
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(attempt.ChildSessionDir, entry.Name())
		child, prefix, err := validateForkChild(path, attempt)
		if err != nil || child.SessionID() == "" || len(prefix) == 0 {
			continue
		}
		candidates = append(candidates, path)
	}
	if len(candidates) != 1 {
		return "", errors.New("Pi fork outcome is unknown; refusing to repeat it")
	}
	return candidates[0], nil
}

func readSessionParent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxHeaderBytes)
	if !scanner.Scan() {
		return "", ErrInvalidSession
	}
	var header sessionParentHeader
	if json.Unmarshal(scanner.Bytes(), &header) != nil || header.Type != "session" || header.Version != 3 || header.ID == "" || header.ParentSession == "" {
		return "", ErrInvalidSession
	}
	return header.ParentSession, nil
}
