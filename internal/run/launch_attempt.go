package run

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/processidentity"
)

const (
	attemptAllocated  = "allocated"
	attemptSpawned    = "spawned"
	attemptTerminated = "terminated"
)

var attemptIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type launchAttempt struct {
	id  string
	log *ledger.Ledger
}

type attemptAllocation struct {
	RequestDigest string `json:"request_digest"`
}

type attemptSpawn struct {
	Identity processidentity.Identity `json:"identity"`
}

type attemptTermination struct {
	Reason string `json:"reason"`
	Proof  string `json:"proof"`
}

type attemptState struct {
	requestDigest string
	identity      *processidentity.Identity
	terminated    bool
}

func (o *Operation) allocateLaunchAttempt(requestDigest string) (*launchAttempt, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(idBytes)
	root := filepath.Join(o.dir, "launch-attempts")
	if err := makeDurableDirectory(root); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, id)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, err
	}
	if err := syncRunDirectory(root); err != nil {
		return nil, err
	}
	attempt := &launchAttempt{id: id, log: ledger.Open(filepath.Join(dir, "events.jsonl"))}
	if _, err := attempt.log.Append(time.Now().UTC(), attemptAllocated, attemptAllocation{RequestDigest: requestDigest}); err != nil {
		return nil, err
	}
	return attempt, nil
}

func (a *launchAttempt) recordSpawn(identity processidentity.Identity) error {
	_, err := a.log.Append(time.Now().UTC(), attemptSpawned, attemptSpawn{Identity: identity})
	return err
}

func (a *launchAttempt) terminate(reason, proof string) error {
	_, err := a.log.Append(time.Now().UTC(), attemptTerminated, attemptTermination{Reason: reason, Proof: proof})
	return err
}

func (a *launchAttempt) state() (attemptState, error) {
	records, err := a.log.Replay()
	if err != nil {
		return attemptState{}, err
	}
	var state attemptState
	for _, record := range records {
		switch record.Kind {
		case attemptAllocated:
			var value attemptAllocation
			if record.Sequence != 1 || state.requestDigest != "" || json.Unmarshal(record.Data, &value) != nil || len(value.RequestDigest) != 64 {
				return attemptState{}, fmt.Errorf("invalid allocated launch attempt %s", a.id)
			}
			state.requestDigest = value.RequestDigest
		case attemptSpawned:
			var value attemptSpawn
			if record.Sequence != 2 || state.requestDigest == "" || state.identity != nil || state.terminated || json.Unmarshal(record.Data, &value) != nil || !validAttemptIdentity(value.Identity) {
				return attemptState{}, fmt.Errorf("invalid spawned launch attempt %s", a.id)
			}
			identity := value.Identity
			state.identity = &identity
		case attemptTerminated:
			var value attemptTermination
			if state.requestDigest == "" || state.terminated || json.Unmarshal(record.Data, &value) != nil || value.Reason == "" || value.Proof == "" {
				return attemptState{}, fmt.Errorf("invalid terminated launch attempt %s", a.id)
			}
			state.terminated = true
		default:
			return attemptState{}, fmt.Errorf("unknown launch attempt event %q", record.Kind)
		}
	}
	if state.requestDigest == "" {
		return attemptState{}, errors.New("launch attempt has no allocation")
	}
	return state, nil
}

func validAttemptIdentity(identity processidentity.Identity) bool {
	return identity.PID > 0 && identity.PGID > 0 && identity.StartToken != "" && identity.CommandHash != "" && identity.Executable != ""
}

func makeDurableDirectory(path string) error {
	err := os.Mkdir(path, 0o700)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return syncRunDirectory(filepath.Dir(path))
}

func syncRunDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
