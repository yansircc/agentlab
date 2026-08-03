package deployctlfixture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

func (f Fixture) resetFiles() error {
	for _, name := range []string{"audit", "receipts", "state"} {
		if err := os.RemoveAll(filepath.Join(f.root, name)); err != nil {
			return err
		}
	}
	for _, name := range []string{"audit", "receipts", "state"} {
		if err := os.Mkdir(filepath.Join(f.root, name), 0o700); err != nil {
			return err
		}
	}
	data, err := json.Marshal(struct {
		Targets []Target `json:"targets"`
	}{Targets: f.targets})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.root, "catalog.json"), data, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.root, "default-target"), []byte(defaultTarget), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.root, "audit", "default-read-count"), []byte("0"), 0o600); err != nil {
		return err
	}
	for _, target := range f.targets {
		if err := os.WriteFile(f.statePath(target.Name), []byte(f.initial[target.Name]), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (f Fixture) catalogDigest() string {
	data, _ := json.Marshal(struct {
		Targets []Target `json:"targets"`
	}{Targets: f.targets})
	return digest(data)
}

func (f Fixture) stateDigest(values map[string]string) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	data := make([]byte, 0, 256)
	for _, name := range names {
		data = append(data, name...)
		data = append(data, 0)
		data = append(data, values[name]...)
		data = append(data, 0)
	}
	return digest(data)
}

func (f Fixture) statePath(target string) string {
	return filepath.Join(f.root, "state", target+".txt")
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fileIsRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("deployctl fixture file is invalid")
	}
	return nil
}
