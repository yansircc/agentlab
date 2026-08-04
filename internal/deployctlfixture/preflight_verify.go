package deployctlfixture

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/metaaudit"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/tool"
)

// Verify replays every provisioned owner and checks the cross-root and exact
// executable boundaries. It intentionally cannot turn provisioning into a
// successful live acceptance claim.
func (value Preflight) Verify() error {
	if err := value.verifyProvision(); err != nil {
		return err
	}
	return value.verifyRuntime()
}

func (value Preflight) verifyRuntime() error {
	if value.FixtureReset != (artifact.Ref{}) || value.Inputs != (experiment.RunInputs{}) {
		if !value.FixtureReset.Valid() || !value.LiveCanary.Valid() || !value.CoderPrepared.Valid() || value.Inputs.Adapter != value.LiveCanary || value.runtimePlanPath == "" {
			return errors.New("deployctl runtime preflight is incomplete")
		}
		host, err := tool.LoadPiRuntimeHost(value.runtimePlanPath)
		if err != nil {
			return errors.New("deployctl runtime plan is invalid")
		}
		identity, err := verifyLiveCanary(artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts")), value.LiveCanary)
		if err != nil {
			return err
		}
		profile, err := host.Profile("baseline-worker")
		if err != nil {
			return errors.New("deployctl Worker runtime profile is absent")
		}
		coderProfile, err := host.Profile("coder-repair")
		if err != nil || profile.WorkerLaunch == nil || coderProfile.CoderLaunch == nil {
			return errors.New("deployctl Coder runtime profile is absent")
		}
		discovered, err := piadapter.VerifyRuntimeIdentity(profile.Identity)
		if err != nil || !reflect.DeepEqual(discovered, identity) {
			return errors.New("deployctl runtime identity differs from live canary")
		}
		supervisor, err := tool.LoadPiSupervisorPlan(value.supervisorPlanPath)
		if err != nil || supervisor.Binding.Root != value.EvaluatedRoot || supervisor.Binding.PreparationID != value.PreparationID || supervisor.Binding.ExperimentID != value.ExperimentID || supervisor.Binding.RuntimePlanPath != value.runtimePlanPath || supervisor.Launch.RuntimeRoot != filepath.Join(value.hostRoot, "supervisor-runtime") || supervisor.SessionPath != filepath.Join(value.hostRoot, "supervisor-runtime", "session.jsonl") || !reflect.DeepEqual(supervisor.Identity, profile.Identity) {
			return errors.New("deployctl Supervisor runtime plan differs from preflight")
		}
		if !verifyRuntimeCredentialIsolation(profile.WorkerLaunch.Launch, *coderProfile.CoderLaunch, supervisor.Launch) {
			return errors.New("deployctl provider credential profiles are not isolated")
		}
		binding, err := loadRuntimeBinding(artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts")), value.Inputs.WorkerRuntime)
		if err != nil || binding.Adapter != value.LiveCanary || binding.CandidateExecutable != value.CandidateExecutable {
			return errors.New("deployctl runtime binding differs from preflight")
		}
		coderPrepared, err := experiment.LoadPreparedRun(artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts")), value.CoderPrepared)
		if err != nil || coderPrepared.RunID != coderRunID || coderPrepared.Inputs.Candidate != value.Candidate || coderPrepared.Inputs.Adapter != value.LiveCanary {
			return errors.New("deployctl Coder prepared run differs from preflight")
		}
		return value.verifyManifest()
	}
	return errors.New("deployctl runtime preflight is absent")
}

func (value Preflight) verifyProvision() error {
	if err := value.verifyRecordedRoots(); err != nil {
		return err
	}
	audit, err := metaaudit.Open(value.AuditRoot, value.AuditID)
	if err != nil {
		return err
	}
	auditStatus, err := audit.Status()
	if err != nil || auditStatus.Sealed || auditStatus.Intervened || len(auditStatus.FindingIDs) != 0 {
		return errors.New("deployctl audit preflight differs")
	}
	return nil
}

func (value Preflight) verifyRecordedRoots() error {
	if !disjointRoots(value.EvaluatedRoot, value.AuditRoot) {
		return errors.New("deployctl preflight roots overlap")
	}
	store := artifact.NewStore(filepath.Join(value.EvaluatedRoot, "artifacts"))
	auditStore := artifact.NewStore(filepath.Join(value.AuditRoot, "artifacts"))
	if _, err := store.Read(value.GroundTruth); err == nil {
		return errors.New("deployctl ground truth crossed into evaluated root")
	}
	if _, err := auditStore.Read(value.Candidate); err == nil {
		return errors.New("deployctl candidate crossed into audit root")
	}
	if err := VerifyBuild(store, value.CandidateExecutable, value.Candidate, filepath.Join(value.EvaluatedRoot, "baseline-candidate", "bin", "deployctl")); err != nil {
		return err
	}
	prep, err := preparation.Open(value.EvaluatedRoot, value.PreparationID)
	if err != nil {
		return err
	}
	status, err := prep.Status()
	if err != nil || status.Phase != preparation.PhaseSealed || status.WorkerInput != value.WorkerInput || status.Source != value.SourceSnapshot {
		return errors.New("deployctl preparation differs from preflight")
	}
	if err := verifyPublicInput(store, status); err != nil {
		return err
	}
	audit, err := metaaudit.Open(value.AuditRoot, value.AuditID)
	if err != nil {
		return err
	}
	auditStatus, err := audit.Status()
	if err != nil || auditStatus.Trial.ExperimentID != value.ExperimentID || auditStatus.Trial.EvaluatedScope != store.Scope() || auditStatus.Trial.GroundTruth != value.GroundTruth {
		return errors.New("deployctl audit preflight differs")
	}
	return nil
}

func (value Preflight) verifyManifest() error {
	op, err := experiment.Open(value.EvaluatedRoot, value.ExperimentID)
	if err != nil {
		return err
	}
	manifest, _, err := op.RunManifest(value.BaselineRunID)
	if err != nil || !manifest.Origin.IsFresh() || manifest.Candidate != value.Candidate || manifest.RunInputs != value.Inputs {
		return errors.New("deployctl baseline manifest differs from preflight")
	}
	return nil
}

func sealPublicInput(store artifact.Store, prep *preparation.Operation, status preparation.Status) error {
	if err := verifyPublicInput(store, status); err != nil {
		return err
	}
	evidence, err := store.Put([]byte("agentlab.deployctl-input-isolation-review.v1"))
	if err != nil {
		return err
	}
	assay := preparation.LeakageAssay{WorkerInput: status.WorkerInput, SourceSnapshot: status.Source, Reviewer: "deployctl-input-auditor", Authority: "reviewer", Method: "exact-rendered-input-review", Verdict: preparation.LeakageClean, Evidence: []artifact.Ref{evidence}}
	if err := prep.RecordLeakageAssay(assay); err != nil {
		return err
	}
	basis, err := prep.ChallengeBasis()
	if err != nil {
		return err
	}
	if err := prep.Challenge(preparation.Challenge{Basis: basis}); err != nil {
		return err
	}
	_, err = prep.Seal()
	return err
}

func verifyPublicInput(store artifact.Store, status preparation.Status) error {
	rendered, err := preparation.RenderWorkerInput(store, status.WorkerInput)
	if err != nil {
		return err
	}
	if strings.Contains(rendered, string(privateFact)) || strings.Contains(rendered, "defaultTarget(") || strings.Contains(rendered, "actual, err := defaultTarget") {
		return errors.New("deployctl worker input leaked private implementation")
	}
	snapshot, err := source.Load(store, status.Source)
	if err != nil {
		return err
	}
	for _, file := range snapshot.Files {
		data, err := store.Read(file.Artifact)
		if err != nil || strings.Contains(rendered, string(data)) {
			return errors.New("deployctl worker input leaked source snapshot")
		}
	}
	return nil
}

func newDisjointRoots(left, right string) bool {
	if !disjointRoots(left, right) {
		return false
	}
	_, leftErr := os.Lstat(left)
	_, rightErr := os.Lstat(right)
	return errors.Is(leftErr, os.ErrNotExist) && errors.Is(rightErr, os.ErrNotExist)
}

func disjointRoots(left, right string) bool {
	return filepath.IsAbs(left) && filepath.IsAbs(right) && left != right && !strings.HasPrefix(left, right+string(filepath.Separator)) && !strings.HasPrefix(right, left+string(filepath.Separator))
}

func putCanonical(store artifact.Store, value any) (artifact.Ref, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.PutCanonicalJSON(data)
}
