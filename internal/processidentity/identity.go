package processidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type Identity struct {
	PID         int    `json:"pid"`
	PGID        int    `json:"pgid"`
	StartToken  string `json:"start_token"`
	CommandHash string `json:"command_hash"`
	Executable  string `json:"executable"`
}

type Observation string

const (
	Matches  Observation = "matches"
	Dead     Observation = "dead"
	Mismatch Observation = "mismatch"
	Unknown  Observation = "unknown"
)

type Prober interface{ Observe(Identity) Observation }

type SystemProber struct{}

func Capture(pid, pgid int, executable string) (Identity, error) {
	start, command, err := psFacts(pid)
	if err != nil {
		return Identity{}, err
	}
	return Identity{PID: pid, PGID: pgid, StartToken: start, CommandHash: hash(command), Executable: executable}, nil
}

func CaptureProcess(pid int) (Identity, error) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return Identity{}, err
	}
	start, command, err := psFacts(pid)
	if err != nil {
		return Identity{}, err
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return Identity{}, fmt.Errorf("process %d has no executable", pid)
	}
	return Identity{PID: pid, PGID: pgid, StartToken: start, CommandHash: hash(command), Executable: fields[0]}, nil
}

func (SystemProber) Observe(identity Identity) Observation {
	start, _, err := psFacts(identity.PID)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Dead
		}
		return Unknown
	}
	// Node-based roles rewrite their own argv (the pinned Pi CLI sets
	// process.title, which replaces the process command line), so the command
	// string is not a stable identity fact. The process start time and the
	// resolved executable are stable and still distinguish a recycled PID.
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(identity.PID), "exe"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Dead
		}
		return Unknown
	}
	if deleted := " (deleted)"; strings.HasSuffix(exe, deleted) {
		exe = strings.TrimSuffix(exe, deleted)
	}
	resolved := identity.Executable
	if !filepath.IsAbs(resolved) {
		if found, err := exec.LookPath(resolved); err == nil {
			resolved = found
		}
	}
	if evaluated, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = evaluated
	}
	if start != identity.StartToken || exe != resolved {
		return Mismatch
	}
	return Matches
}

func psFacts(pid int) (string, string, error) {
	out, err := exec.Command("ps", "-ww", "-p", strconv.Itoa(pid), "-o", "lstart=", "-o", "command=").Output()
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 6 {
		return "", "", fmt.Errorf("unexpected ps output for pid %d", pid)
	}
	return strings.Join(fields[:5], " "), strings.Join(fields[5:], " "), nil
}

// Alive reports whether the named process identity still exists with the same
// start time and executable. It is the conservative staleness probe for
// transaction leases: a lease whose holder is provably gone is stale.
func Alive(identity Identity) (bool, error) {
	return SystemProber{}.Observe(identity) == Matches, nil
}

// hash seals the captured command line for receipt identity. It is a stable
// snapshot of the spawn, not a live comparison: node-based roles rewrite
// their own argv after start, so Observe compares start time and executable.
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
