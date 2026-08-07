package run

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/processidentity"
)

const managedAttachedAttemptContract = "agentlab.managed-adapter-start.v1"

type ManagedAttachedSpec struct {
	Adapter          string
	Policy           StopPolicy
	Capabilities     AdapterCapabilities
	Command          []string
	Environment      []string
	WorkingDirectory string
	Ready            func() (string, []byte, error)
	Coder            *CoderProfile
	Finalize         func(exitCode int) error
}

type managedAttachedAttempt struct {
	Contract      string              `json:"contract"`
	Adapter       string              `json:"adapter"`
	Policy        StopPolicy          `json:"policy"`
	Capabilities  AdapterCapabilities `json:"capabilities"`
	RequestDigest string              `json:"request_digest"`
}

func (o *Operation) BeginManagedAttachedEffect(intent effect.Intent, spec ManagedAttachedSpec) (AttachedStartResult, error) {
	if intent.RunID != o.runID || (intent.Kind != effect.WorkerStart && intent.Kind != effect.CoderStart) || intent.Validate() != nil || validateManagedAttached(spec) != nil || o.validateManagedRole(intent, spec) != nil {
		return AttachedStartResult{}, errors.New("managed adapter start effect is invalid")
	}
	payload, err := o.startPayload(intent)
	if err != nil {
		return AttachedStartResult{}, err
	}
	manifest, err := o.requireManifest()
	if err != nil {
		return AttachedStartResult{}, err
	}
	attemptData, err := json.Marshal(managedAttachedAttempt{Contract: managedAttachedAttemptContract, Adapter: spec.Adapter, Policy: spec.Policy, Capabilities: spec.Capabilities, RequestDigest: managedDigest(spec)})
	if err != nil {
		return AttachedStartResult{}, err
	}
	created, err := o.BeginEffectAttempt(intent, attemptData)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if !created {
		return o.reconcileManagedAttachedStart(intent)
	}
	if err := o.reconcileLaunchAttempts(); err != nil {
		return AttachedStartResult{}, err
	}
	return o.startManagedAttached(intent, payload, manifest, spec)
}

func (o *Operation) startManagedAttached(intent effect.Intent, payload StartPayload, manifest artifact.Ref, spec ManagedAttachedSpec) (AttachedStartResult, error) {
	attempt, err := o.allocateLaunchAttempt(managedDigest(spec))
	if err != nil {
		return AttachedStartResult{}, err
	}
	command := exec.Command(spec.Command[0], spec.Command[1:]...)
	command.Dir, command.Env = spec.WorkingDirectory, append([]string(nil), spec.Environment...)
	command.Stdin = nil
	// Managed role output is normally discarded; the session file is the
	// record. For Host-side diagnosis only, AGENTLAB_MANAGED_STDERR names a
	// file that receives the managed process stderr so a startup failure is
	// observable instead of silent.
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if capture := os.Getenv("AGENTLAB_MANAGED_STDERR"); capture != "" {
		if file, err := os.OpenFile(capture, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			command.Stderr = file
			defer file.Close()
		}
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = attempt.terminate("spawn_failed", "no_process_created")
		return AttachedStartResult{}, err
	}
	pgid, err := syscall.Getpgid(command.Process.Pid)
	if err != nil {
		terminateUnrecorded(command, command.Process.Pid)
		_ = attempt.terminate("pgid_capture_failed", "process_killed_and_waited")
		return AttachedStartResult{}, err
	}
	identity, err := processidentity.Capture(command.Process.Pid, pgid, spec.Command[0])
	if err != nil {
		terminateUnrecorded(command, pgid)
		_ = attempt.terminate("identity_capture_failed", "process_group_killed_and_waited")
		return AttachedStartResult{}, err
	}
	if err := o.recordAttemptSpawn(attempt, identity); err != nil {
		terminateUnrecorded(command, pgid)
		_ = attempt.terminate("identity_receipt_failed", "process_group_killed_and_waited")
		return AttachedStartResult{}, err
	}
	streamID, cursor, err := spec.Ready()
	if err != nil {
		terminateUnrecorded(command, pgid)
		_ = attempt.terminate("adapter_not_ready", "process_group_killed_and_waited")
		return AttachedStartResult{}, err
	}
	cursorRef, err := o.artifacts.Put(cursor)
	if err != nil {
		terminateUnrecorded(command, pgid)
		_ = attempt.terminate("cursor_persist_failed", "process_group_killed_and_waited")
		return AttachedStartResult{}, err
	}
	started := processStarted{AttemptID: attempt.id, Manifest: manifest, Process: processHandle{Kind: processManaged, Identity: &identity}, Policy: spec.Policy, Adapter: &adapterBinding{Adapter: spec.Adapter, StreamID: streamID, Cursor: cursorRef, Capabilities: spec.Capabilities}, Coder: spec.Coder}
	if _, err := o.appendEvent(time.Now().UTC(), eventProcessStarted, started); err != nil {
		terminateUnrecorded(command, pgid)
		_ = attempt.terminate("start_event_failed", "process_group_killed_and_waited")
		return AttachedStartResult{}, err
	}
	state := AdapterState{Adapter: spec.Adapter, StreamID: streamID, Cursor: cursor}
	evidence, err := encodeStartObservation(state, payload)
	if err != nil || o.RecordEffectObservation(intent, evidence) != nil {
		go o.awaitManaged(command, spec.Finalize)
		return AttachedStartResult{}, errors.New("managed adapter start observation failed")
	}
	result, err := o.settleAttachedStart(intent, evidence)
	go o.awaitManaged(command, spec.Finalize)
	return result, err
}

func (o *Operation) reconcileManagedAttachedStart(intent effect.Intent) (AttachedStartResult, error) {
	evidence, exists, err := o.EffectObservation(intent)
	if err != nil {
		return AttachedStartResult{}, err
	}
	if exists {
		return o.settleAttachedStart(intent, evidence)
	}
	if err := o.reconcileLaunchAttempts(); err != nil {
		return AttachedStartResult{}, err
	}
	return AttachedStartResult{}, errors.New("managed adapter start outcome is unknown; refusing to repeat it")
}

func validateManagedAttached(spec ManagedAttachedSpec) error {
	if spec.Adapter == "" || spec.Ready == nil || spec.Finalize == nil || !filepath.IsAbs(spec.WorkingDirectory) || len(spec.Command) == 0 || !filepath.IsAbs(spec.Command[0]) || !spec.Policy.OwnsWorkerProcess || spec.Policy.Validate() != nil || spec.Capabilities != RequiredAdapterCapabilities() {
		return errors.New("managed adapter specification is invalid")
	}
	seen := map[string]bool{}
	for _, value := range spec.Environment {
		key, _, found := strings.Cut(value, "=")
		if !found || key == "" || seen[key] {
			return errors.New("managed adapter environment is invalid")
		}
		seen[key] = true
	}
	return nil
}

func (o *Operation) validateManagedRole(intent effect.Intent, spec ManagedAttachedSpec) error {
	if intent.Kind == effect.WorkerStart && spec.Coder == nil {
		return nil
	}
	if intent.Kind != effect.CoderStart || spec.Coder == nil {
		return errors.New("managed Pi role is invalid")
	}
	profile, err := o.CoderProfile(intent)
	if err != nil || profile != *spec.Coder {
		return errors.New("managed Coder profile differs from start intent")
	}
	return nil
}

func managedDigest(spec ManagedAttachedSpec) string {
	data, _ := json.Marshal(struct {
		Command, Environment []string
		WorkingDirectory     string
	}{spec.Command, spec.Environment, spec.WorkingDirectory})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
