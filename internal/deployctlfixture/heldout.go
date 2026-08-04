package deployctlfixture

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
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
	profile, err := host.Profile(runtime.WorkerProfile)
	if err != nil || profile.Role != effect.WorkerStart || profile.RunID != prepared.RunID || profile.WorkerLaunch == nil || profile.WorkerLaunch.CandidateExecutable != runtime.CandidateExecutable || run.VerifyCandidateExecutable(store, runtime.CandidateExecutable, prepared.Inputs.Candidate, profile.WorkerLaunch.DeployctlExecutable) != nil {
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

func heldoutNonce() (string, error) {
	bytes := make([]byte, 16)
	if _, err := cryptorand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
