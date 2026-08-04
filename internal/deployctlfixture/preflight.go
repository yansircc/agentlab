package deployctlfixture

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/metaaudit"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/source"
)

const (
	preparationID = "deployctl-preparation"
	experimentID  = "deployctl-supervision"
	baselineRunID = "baseline-worker"
	coderRunID    = "coder-repair"
	auditID       = "deployctl-meta-audit"
)

var (
	workerTask  = []byte("Use the public deployctl CLI to deploy release-a to staging. Verify staging serves release-a, production remains unchanged, and leave a deployment receipt.")
	publicHelp  = []byte("deployctl deploy --target <target> --release <release>; deployctl status --target <target>; deployctl receipt")
	privateFact = []byte("AGENTLAB_PRIVATE_DEPLOYCTL_TARGET_FAULT: deploy execution re-reads the default target after target parsing")
)

// PreflightSpec names only Host-owned, fresh capability roots. The pack owns
// all task ids and public task material, so callers cannot alter the trial by
// selecting a different preparation, experiment, or baseline run.
type PreflightSpec struct {
	EvaluatedRoot string
	AuditRoot     string
}

// Preflight is the Host-only assembly of the controlled deployctl task. None
// of its paths or audit refs belongs in a WorkerInput or four-tool response.
// It provisions facts only; it does not claim the live Pi Stage 0 canary or
// any Worker/Supervisor acceptance has passed.
type Preflight struct {
	EvaluatedRoot       string
	AuditRoot           string
	PreparationID       string
	ExperimentID        string
	BaselineRunID       string
	AuditID             string
	Fixture             Fixture
	WorkerInput         artifact.Ref
	SourceSnapshot      artifact.Ref
	Candidate           artifact.Ref
	CandidateExecutable artifact.Ref
	FixtureReset        artifact.Ref
	LiveCanary          artifact.Ref
	Inputs              experiment.RunInputs
	GroundTruth         artifact.Ref
	reset               ResetReceipt
	hostRoot            string
	runtimePlanPath     string
}

// ProvisionPreflight creates the two disjoint roots, seals the exact public
// Worker input, binds a fresh baseline run, and opens the independent audit
// ledger. Existing roots are rejected so a failed or prior trial cannot be
// silently reused.
func ProvisionPreflight(spec PreflightSpec) (Preflight, error) {
	if !newDisjointRoots(spec.EvaluatedRoot, spec.AuditRoot) {
		return Preflight{}, errors.New("deployctl preflight roots are invalid")
	}
	if err := os.Mkdir(spec.EvaluatedRoot, 0o700); err != nil {
		return Preflight{}, err
	}
	if err := os.Mkdir(spec.AuditRoot, 0o700); err != nil {
		return Preflight{}, err
	}
	store := artifact.NewStore(filepath.Join(spec.EvaluatedRoot, "artifacts"))
	fixture, err := New(filepath.Join(spec.EvaluatedRoot, "fixture"))
	if err != nil {
		return Preflight{}, err
	}
	reset, err := fixture.Reset()
	if err != nil {
		return Preflight{}, err
	}
	prep, err := preparation.Open(spec.EvaluatedRoot, preparationID)
	if err != nil {
		return Preflight{}, err
	}
	status, err := prep.Begin(preparation.BeginSpec{UserIntent: workerTask, SourceFiles: BaselineSource(), PublicArtifacts: [][]byte{publicHelp}, Authority: "deployctl-preflight-host"})
	if err != nil {
		return Preflight{}, err
	}
	if err := sealPublicInput(store, prep, status); err != nil {
		return Preflight{}, err
	}
	status, err = prep.Status()
	if err != nil || status.Phase != preparation.PhaseSealed {
		return Preflight{}, errors.New("deployctl preflight input was not sealed")
	}
	if _, err := source.Load(store, status.Source); err != nil {
		return Preflight{}, err
	}
	workspace := filepath.Join(spec.EvaluatedRoot, "baseline-candidate")
	executable := filepath.Join(workspace, "bin", "deployctl")
	candidateExecutable, err := BuildCandidate(store, status.Source, workspace, executable)
	if err != nil {
		return Preflight{}, err
	}
	auditStore := artifact.NewStore(filepath.Join(spec.AuditRoot, "artifacts"))
	groundTruth, err := auditStore.Put(privateFact)
	if err != nil {
		return Preflight{}, err
	}
	audit, err := metaaudit.Open(spec.AuditRoot, auditID)
	if err != nil {
		return Preflight{}, err
	}
	if err := audit.Begin(metaaudit.Trial{Contract: metaaudit.Contract, ExperimentID: experimentID, EvaluatedScope: store.Scope(), GroundTruth: groundTruth}); err != nil {
		return Preflight{}, err
	}
	result := Preflight{
		EvaluatedRoot: spec.EvaluatedRoot, AuditRoot: spec.AuditRoot, PreparationID: preparationID, ExperimentID: experimentID, BaselineRunID: baselineRunID, AuditID: auditID,
		Fixture: fixture, WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Candidate: status.Source, CandidateExecutable: candidateExecutable,
		GroundTruth: groundTruth, reset: reset,
	}
	return result, result.verifyProvision()
}
