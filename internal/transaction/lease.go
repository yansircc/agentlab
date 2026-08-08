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
	"time"

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
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
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
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		// A crashed holder leaves the lock file behind; the receipt names its
		// process identity, so a dead holder is provably stale and may be
		// broken and retried instead of failing closed forever.
		stale, err := staleLease(path)
		if err != nil {
			return nil, err
		}
		if !stale {
			return nil, ErrLeaseHeld
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
		if err := syncDir(filepath.Dir(path)); err != nil {
			return nil, err
		}
	}
}

func staleLease(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var current receipt
	if json.Unmarshal(data, &current) != nil {
		// An unreadable receipt cannot prove a live holder; break it only if
		// the file is older than any plausible transaction window.
		info, statErr := os.Stat(path)
		if statErr != nil || time.Since(info.ModTime()) < 5*time.Minute {
			return false, nil
		}
		return true, nil
	}
	// A lease whose named process is gone is stale: processidentity.Capture
	// records the holder's pid and exe, and /proc recycles pids only after the
	// process's start time changes, which the identity records too.
	alive, err := processidentity.Alive(current.Identity)
	if err != nil {
		return false, err
	}
	return !alive, nil
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
