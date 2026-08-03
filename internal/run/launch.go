package run

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/processidentity"
)

type requestReceipt struct {
	Executable               string            `json:"executable"`
	CommandHash              string            `json:"command_hash"`
	Policy                   StopPolicy        `json:"policy"`
	Manifest                 artifact.Ref      `json:"manifest"`
	PublicEnvironment        map[string]string `json:"public_environment,omitempty"`
	SecretEnvironmentHandles map[string]string `json:"secret_environment_handles,omitempty"`
}

type ownedWorker struct {
	command  *exec.Cmd
	identity processidentity.Identity
	stdout   io.ReadCloser
	stderr   io.ReadCloser
}

func (o *Operation) bindOwnedRequest(spec StartSpec, manifest artifact.Ref) (string, error) {
	commandBytes, _ := json.Marshal(spec.PublicCommand)
	sum := sha256.Sum256(commandBytes)
	receipt := requestReceipt{Executable: spec.PublicCommand[0], CommandHash: hex.EncodeToString(sum[:]), Policy: spec.Policy, Manifest: manifest, PublicEnvironment: spec.PublicEnvironment, SecretEnvironmentHandles: spec.SecretEnvironmentHandles}
	if err := o.writeReceipt("request.json", receipt); err != nil {
		return "", err
	}
	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(receiptBytes)
	return hex.EncodeToString(digest[:]), nil
}

func (o *Operation) launchOwned(spec StartSpec, requestDigest string, manifest artifact.Ref, environment []string) (*ownedWorker, error) {
	attempt, err := o.allocateLaunchAttempt(requestDigest)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(spec.PublicCommand[0], spec.PublicCommand[1:]...)
	cmd.Env = append([]string(nil), environment...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		if journalErr := attempt.terminate("spawn_failed", "no_process_created"); journalErr != nil {
			return nil, errors.Join(err, journalErr)
		}
		return nil, err
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		journalErr := attempt.terminate("pgid_capture_failed", "process_killed_and_waited")
		return nil, errors.Join(err, journalErr)
	}
	identity, err := processidentity.Capture(cmd.Process.Pid, pgid, spec.PublicCommand[0])
	if err != nil {
		terminateUnrecorded(cmd, pgid)
		journalErr := attempt.terminate("identity_capture_failed", "process_group_killed_and_waited")
		return nil, errors.Join(fmt.Errorf("capture worker identity: %w", err), journalErr)
	}
	if err := o.recordAttemptSpawn(attempt, identity); err != nil {
		terminateUnrecorded(cmd, pgid)
		journalErr := attempt.terminate("identity_receipt_failed", "process_group_killed_and_waited")
		return nil, errors.Join(err, journalErr)
	}
	started := processStarted{AttemptID: attempt.id, Manifest: manifest, Process: processHandle{Kind: processOwned, Identity: &identity}, Policy: spec.Policy}
	worker := &ownedWorker{command: cmd, identity: identity, stdout: stdout, stderr: stderr}
	if _, err := o.appendEvent(time.Now().UTC(), eventProcessStarted, started); err != nil {
		return o.resolveStartAppendError(worker, attempt, started, err)
	}
	return worker, nil
}

func terminateUnrecorded(cmd *exec.Cmd, pgid int) {
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	_, _ = cmd.Process.Wait()
}
