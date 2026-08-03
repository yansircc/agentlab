package run

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
)

func (o *Operation) startOwned(ctx context.Context, runID string, spec StartSpec, manifest artifact.Ref) (StartResult, error) {
	if len(spec.PublicCommand) == 0 {
		return StartResult{}, errors.New("worker command is required")
	}
	if !filepath.IsAbs(spec.PublicCommand[0]) {
		return StartResult{}, errors.New("worker executable must be absolute")
	}
	environment, err := resolveEnvironment(spec.PublicEnvironment, spec.SecretEnvironmentHandles)
	if err != nil {
		return StartResult{}, err
	}
	if err := spec.Policy.Validate(); err != nil {
		return StartResult{}, err
	}
	if !spec.Policy.OwnsWorkerProcess {
		return StartResult{}, errors.New("owned runner requires owns_worker_process")
	}
	requestDigest, err := o.bindOwnedRequest(spec, manifest)
	if err != nil {
		return StartResult{}, err
	}
	if err := o.reconcileLaunchAttempts(); err != nil {
		return StartResult{}, err
	}
	worker, err := o.launchOwned(spec, requestDigest, manifest, environment.values)
	if err != nil {
		return StartResult{}, err
	}

	items := make(chan streamItem)
	go scanStream("stdout", worker.stdout, items)
	go scanStream("stderr", worker.stderr, items)

	firstTimer := time.NewTimer(spec.Policy.FirstEventTimeout)
	softTimer := time.NewTimer(spec.Policy.SoftIdleTimeout)
	hardTimer := time.NewTimer(spec.Policy.HardIdleTimeout)
	stopTicker := time.NewTicker(100 * time.Millisecond)
	defer stopTimer(firstTimer)
	defer stopTimer(softTimer)
	defer stopTimer(hardTimer)
	defer stopTicker.Stop()
	firstC, softC, hardC := firstTimer.C, softTimer.C, hardTimer.C
	firstSeen := false
	closed := 0
	corrupt := false
	var candidates [][]byte
	terminated := false
	var forceTimer *time.Timer

	for closed < 2 {
		select {
		case item := <-items:
			if item.eof {
				closed++
				if item.err != nil {
					corrupt = true
					if _, err := o.appendEvent(time.Now().UTC(), eventStreamCorrupt, streamFact{Stream: item.stream, Error: item.err.Error()}); err != nil {
						return StartResult{}, err
					}
				}
				if _, err := o.appendEvent(time.Now().UTC(), eventStreamClosed, streamFact{Stream: item.stream}); err != nil {
					return StartResult{}, err
				}
				continue
			}
			line := redactSecrets(item.line, environment.secrets)
			ref, err := o.artifacts.Put(line)
			if err != nil {
				return StartResult{}, err
			}
			label := classify(line)
			if label == "terminal_candidate" {
				candidates = append(candidates, append([]byte(nil), line...))
			}
			if _, err := o.appendEvent(time.Now().UTC(), eventEvidence, evidence{Stream: item.stream, Raw: ref, Label: label}); err != nil {
				return StartResult{}, err
			}
			if !firstSeen {
				firstSeen = true
				stopTimer(firstTimer)
				firstC = nil
			}
			resetTimer(softTimer, spec.Policy.SoftIdleTimeout)
			resetTimer(hardTimer, spec.Policy.HardIdleTimeout)
			softC, hardC = softTimer.C, hardTimer.C
		case <-firstC:
			if _, err := o.appendEvent(time.Now().UTC(), eventFirstTimeout, struct{}{}); err != nil {
				return StartResult{}, err
			}
			firstC = nil
		case <-softC:
			if _, err := o.appendEvent(time.Now().UTC(), eventSoftIdle, struct{}{}); err != nil {
				return StartResult{}, err
			}
			softC = nil
		case <-hardC:
			if _, err := o.appendEvent(time.Now().UTC(), eventHardIdle, struct{}{}); err != nil {
				return StartResult{}, err
			}
			hardC = nil
			if spec.Policy.KillOnHardIdle && !terminated {
				terminated = true
				forceTimer = terminateOwned(worker.identity.PGID)
			}
		case <-ctx.Done():
			if !terminated {
				if _, err := o.admitOwnedStop(ctx.Err().Error()); err != nil {
					return StartResult{}, err
				}
				terminated = true
				forceTimer = terminateOwned(worker.identity.PGID)
			}
		case <-stopTicker.C:
			if !terminated {
				request, err := o.admitOwnedStop("")
				if err != nil {
					return StartResult{}, err
				}
				if request != nil {
					terminated = true
					forceTimer = terminateOwned(worker.identity.PGID)
				}
			}
		}
	}

	waitErr := worker.command.Wait()
	if forceTimer != nil {
		forceTimer.Stop()
	}
	code := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return StartResult{}, waitErr
		}
		code = exitErr.ExitCode()
	}
	if _, err := o.appendEvent(time.Now().UTC(), eventProcessExited, processExited{Code: code}); err != nil {
		return StartResult{}, err
	}
	result, validationErr := validateTerminal(code, corrupt, candidates)
	if validationErr != nil {
		if _, err := o.appendEvent(time.Now().UTC(), eventTerminalRejected, terminalRejected{Reason: validationErr.Error()}); err != nil {
			return StartResult{}, err
		}
		return StartResult{RunID: runID, Code: code}, validationErr
	} else if _, err := o.appendEvent(time.Now().UTC(), eventTerminalAccepted, result); err != nil {
		return StartResult{}, err
	}
	return StartResult{RunID: runID, Code: code}, nil
}

func terminateOwned(pgid int) *time.Timer {
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	return time.AfterFunc(2*time.Second, func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
}
