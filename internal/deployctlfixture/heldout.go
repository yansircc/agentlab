package deployctlfixture

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
	"github.com/yansircc/agentlab/internal/tool"
)

const heldoutVerificationContract = "agentlab.deployctl-heldout-verification.v1"

// HeldoutVerification is Host-owned objective evidence. It is not a Worker
// run and cannot be used as a fresh autonomous acceptance claim.
type HeldoutVerification struct {
	Contract        string         `json:"contract"`
	Prepared        artifact.Ref   `json:"prepared"`
	Candidate       artifact.Ref   `json:"candidate"`
	Trial           HeldoutReceipt `json:"trial"`
	ExitCode        int            `json:"exit_code"`
	TerminalSuccess bool           `json:"terminal_success"`
	Oracle          OracleResult   `json:"oracle"`
}

// VerifyHeldoutPreparedRun generates a target only after the exact candidate
// is sealed in PreparedRun, then runs that candidate through the public CLI.
// Its artifact is objective mutation-B evidence, never a substitution for a
// fresh Worker trial or comparison run.
func (value Preflight) VerifyHeldoutPreparedRun(preparedRef artifact.Ref) (artifact.Ref, error) {
	if !preparedRef.Valid() {
		return artifact.Ref{}, errors.New("deployctl held-out prepared run is invalid")
	}
	if err := value.verifyRuntime(); err != nil {
		return artifact.Ref{}, errors.New("deployctl runtime preflight is unavailable")
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	prepared, err := experiment.LoadPreparedRun(store, preparedRef)
	if err != nil {
		return artifact.Ref{}, errors.New("deployctl held-out prepared run is invalid")
	}
	runtime, err := loadRuntimeBinding(store, prepared.Inputs.WorkerRuntime)
	if err != nil || runtime.Adapter != value.LiveCanary {
		return artifact.Ref{}, errors.New("deployctl held-out runtime binding is invalid")
	}
	host, err := tool.LoadPiRuntimeHost(value.runtimePlanPath)
	if err != nil {
		return artifact.Ref{}, errors.New("deployctl runtime plan is invalid")
	}
	profile, err := host.PreparedWorker(runtime.WorkerProfile)
	if err != nil || profile.RunID != prepared.RunID || profile.WorkerRuntime != prepared.Inputs.WorkerRuntime || profile.WorkerLaunch.CandidateExecutable != runtime.CandidateExecutable || run.VerifyCandidateExecutable(store, runtime.CandidateExecutable, prepared.Inputs.Candidate, profile.WorkerLaunch.DeployctlExecutable) != nil {
		return artifact.Ref{}, errors.New("deployctl held-out Worker profile differs from prepared run")
	}
	nonce, err := heldoutNonce()
	if err != nil {
		return artifact.Ref{}, err
	}
	fixture, target, trial, err := value.Fixture.Heldout(filepath.Join(value.EvaluatedRoot, "heldout-fixture-"+nonce), prepared.Inputs.Candidate, nonce)
	if err != nil {
		return artifact.Ref{}, err
	}
	command, err := fixture.Execute(profile.WorkerLaunch.DeployctlExecutable, "deploy", "--target", target, "--release", "release-a")
	if err != nil {
		return artifact.Ref{}, err
	}
	oracle, err := fixture.Oracle(target, "release-a")
	if err != nil {
		return artifact.Ref{}, err
	}
	return putCanonical(store, HeldoutVerification{
		Contract: heldoutVerificationContract, Prepared: preparedRef, Candidate: prepared.Inputs.Candidate, Trial: trial,
		ExitCode: command.ExitCode, TerminalSuccess: command.TerminalSuccess(), Oracle: oracle,
	})
}

// VerifyHeldoutArtifact admits only the Host-produced post-seal evidence that
// belongs to the exact candidate closing a recursive gate. It is deliberately
// not a Worker-run projection and cannot supply comparison repetitions.
func VerifyHeldoutArtifact(store artifact.Store, ref, candidate artifact.Ref) error {
	if !ref.Valid() || !candidate.Valid() {
		return errors.New("deployctl held-out verification is invalid")
	}
	data, err := store.Read(ref)
	if err != nil {
		return err
	}
	canonical, err := artifact.CanonicalJSON(data)
	if err != nil || !bytes.Equal(data, canonical) {
		return errors.New("deployctl held-out verification is not canonical")
	}
	var value HeldoutVerification
	if strictjson.Decode(data, &value) != nil || value.Contract != heldoutVerificationContract || value.Candidate != candidate || !value.Prepared.Valid() || value.Trial.Contract != heldoutContract || value.Trial.Candidate != candidate || !strings.HasPrefix(value.Trial.Target, "heldout-") || !validTarget(value.Trial.Target) || value.Trial.Reset.Contract != resetContract || value.Trial.Reset.CatalogDigest == "" || value.Trial.Reset.StateDigest == "" || value.ExitCode != 0 || !value.TerminalSuccess || value.Oracle.Target != value.Trial.Target || value.Oracle.Release != "release-a" || !value.Oracle.Pass() {
		return errors.New("deployctl held-out verification is invalid")
	}
	prepared, err := experiment.LoadPreparedRun(store, value.Prepared)
	if err != nil || prepared.Inputs.Candidate != candidate {
		return errors.New("deployctl held-out verification differs from prepared candidate")
	}
	return nil
}

func heldoutNonce() (string, error) {
	bytes := make([]byte, 16)
	if _, err := cryptorand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
