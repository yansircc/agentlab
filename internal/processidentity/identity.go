package processidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
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
	start, command, err := psFacts(identity.PID)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Dead
		}
		return Unknown
	}
	if start != identity.StartToken || hash(command) != identity.CommandHash {
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

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
