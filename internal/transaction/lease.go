package transaction

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/yansircc/agentlab/internal/processidentity"
)

var (
	ErrLeaseHeld    = errors.New("writer lease is already held")
	ErrLeaseChanged = errors.New("writer lease ownership changed")
)

type receipt struct {
	Token    string                   `json:"token"`
	Identity processidentity.Identity `json:"identity"`
}

type Lease struct {
	path  string
	token string
}

func Acquire(path string) (*Lease, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		return nil, err
	}
	identity, err := processidentity.Capture(os.Getpid(), pgid, os.Args[0])
	if err != nil {
		return nil, err
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(bytes)
	data, err := json.Marshal(receipt{Token: token, Identity: identity})
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil, ErrLeaseHeld
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return &Lease{path: path, token: token}, nil
}

func (l *Lease) Release() error {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return err
	}
	var current receipt
	if json.Unmarshal(data, &current) != nil || current.Token != l.token {
		return ErrLeaseChanged
	}
	if err := os.Remove(l.path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(l.path))
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
