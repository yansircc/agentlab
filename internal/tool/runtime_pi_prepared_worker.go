package tool

import (
	"errors"
	"path/filepath"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/run"
)

// PiPreparedWorkerRuntime is Host-private process authority issued together
// with one PreparedRun. It has no provider-authored input: a fresh manifest
// starts at FreshSessionPath, while a splice manifest can start only after the
// Host has materialized Forked from the settled parent receipt.
//
// The template and any later Forked binding are one runtime profile lifecycle,
// not a second source for manifest inputs. WorkerRuntime is the opaque input
// artifact carried by the Host-issued PreparedRun and later manifest.
type PiPreparedWorkerRuntime struct {
	Ref              string                   `json:"ref"`
	ExperimentID     string                   `json:"experiment_id"`
	RunID            string                   `json:"run_id"`
	WorkerRuntime    artifact.Ref             `json:"worker_runtime"`
	FreshSessionPath string                   `json:"fresh_session_path"`
	Identity         piadapter.IdentityConfig `json:"identity"`
	Policy           run.StopPolicy           `json:"policy"`
	WorkerLaunch     PiWorkerLaunch           `json:"worker_launch"`
	Forked           *PiForkedWorkerBinding   `json:"forked,omitempty"`
}

// PiForkedWorkerBinding is written once by the Host after the fork effect has
// settled. It binds the actual child session to its parent effect receipt and
// the already-bound child manifest. Session paths remain adapter-private.
type PiForkedWorkerBinding struct {
	ParentRun        string            `json:"parent_run"`
	ParentRuntimeRef string            `json:"parent_runtime_ref"`
	ChildManifest    artifact.Ref      `json:"child_manifest"`
	ForkReceipt      effect.Receipt    `json:"fork_receipt"`
	Forked           run.SessionForked `json:"forked"`
}

func (value PiPreparedWorkerRuntime) Validate() error {
	if value.Ref == "" || value.ExperimentID == "" || value.RunID == "" || !value.WorkerRuntime.Valid() || !filepath.IsAbs(value.FreshSessionPath) || !filepath.IsAbs(value.Identity.SDKRoot) || !filepath.IsAbs(value.Identity.ContextFilterPath) || value.Policy.Validate() != nil || !value.Policy.OwnsWorkerProcess || value.Policy.KillOnHardIdle || value.WorkerLaunch.Validate() != nil || !inside(value.WorkerLaunch.Launch.RuntimeRoot, value.FreshSessionPath) || overlaps(value.WorkerLaunch.FixtureRoot, value.Identity.SDKRoot) || overlaps(value.WorkerLaunch.Launch.RuntimeRoot, value.Identity.SDKRoot) {
		return errors.New("prepared Worker runtime is invalid")
	}
	if value.Forked != nil && value.Forked.Validate() != nil {
		return errors.New("prepared Worker fork binding is invalid")
	}
	return nil
}

func (value PiForkedWorkerBinding) Validate() error {
	if value.ParentRun == "" || value.ParentRuntimeRef == "" || !value.ChildManifest.Valid() || value.ForkReceipt.Validate() != nil || value.ForkReceipt.Kind != effect.Fork || value.Forked.Validate() != nil || value.Forked.Intent.ID != value.ForkReceipt.IntentID || value.Forked.Intent.Kind != value.ForkReceipt.Kind {
		return errors.New("forked Worker runtime binding is invalid")
	}
	return nil
}

func (value PiPreparedWorkerRuntime) clone() PiPreparedWorkerRuntime {
	result := value
	result.WorkerLaunch = value.WorkerLaunch.clone()
	if value.Forked != nil {
		copy := *value.Forked
		result.Forked = &copy
	}
	return result
}

func (h *PiRuntimeHost) activeWorkerProfile(binding Binding, runID, ref string) (PiRuntimeProfile, error) {
	if profile, err := h.profile(binding, runID, ref); err == nil {
		if profile.Role != effect.WorkerStart {
			return PiRuntimeProfile{}, errors.New("Pi runtime profile is not a Worker")
		}
		return profile, nil
	}
	template, err := h.preparedWorker(binding, runID, ref)
	if err != nil {
		return PiRuntimeProfile{}, errors.New("Pi Worker runtime is absent")
	}
	return h.resolvePreparedWorker(binding, template)
}

func (h *PiRuntimeHost) resolvePreparedWorker(binding Binding, template PiPreparedWorkerRuntime) (PiRuntimeProfile, error) {
	manifest, manifestRef, err := preparedWorkerManifest(binding, template)
	if err != nil {
		return PiRuntimeProfile{}, err
	}
	if manifest.Origin.IsFresh() {
		return template.freshProfile(), nil
	}
	origin, ok := manifest.Origin.Splice()
	if !ok || template.Forked == nil {
		return PiRuntimeProfile{}, errors.New("splice Worker has no settled fork receipt")
	}
	return h.reconcileForkedPreparedWorker(binding, template, manifestRef, origin)
}

func preparedWorkerManifest(binding Binding, template PiPreparedWorkerRuntime) (experiment.RunManifest, artifact.Ref, error) {
	op, err := binding.experiment()
	if err != nil {
		return experiment.RunManifest{}, artifact.Ref{}, err
	}
	manifest, manifestRef, err := op.RunManifest(template.RunID)
	if err != nil || manifest.WorkerRuntime != template.WorkerRuntime || manifest.WorkerInput != template.WorkerLaunch.WorkerInput || run.VerifyCandidateExecutable(binding.store(), template.WorkerLaunch.CandidateExecutable, manifest.Candidate, template.WorkerLaunch.DeployctlExecutable) != nil {
		return experiment.RunManifest{}, artifact.Ref{}, errors.New("prepared Worker runtime differs from Host-issued manifest")
	}
	return manifest, manifestRef, nil
}

func (value PiPreparedWorkerRuntime) freshProfile() PiRuntimeProfile {
	launch := value.WorkerLaunch.clone()
	return PiRuntimeProfile{
		Ref: value.Ref, ExperimentID: value.ExperimentID, RunID: value.RunID, Role: effect.WorkerStart,
		SessionPath: value.FreshSessionPath, Identity: value.Identity,
		Policy: value.Policy, WorkerLaunch: &launch,
	}
}

func (h *PiRuntimeHost) reconcileForkedPreparedWorker(binding Binding, template PiPreparedWorkerRuntime, manifestRef artifact.Ref, origin experiment.SpliceOriginSpec) (PiRuntimeProfile, error) {
	forked := *template.Forked
	if forked.ParentRun != origin.ParentRun || forked.ChildManifest != manifestRef || forked.Forked.ExpectedCheckpoint != origin.RuntimeCheckpoint {
		return PiRuntimeProfile{}, errors.New("forked Worker runtime differs from splice manifest")
	}
	parent, err := h.activeWorkerProfile(binding, origin.ParentRun, forked.ParentRuntimeRef)
	if err != nil || parent.Identity != template.Identity {
		return PiRuntimeProfile{}, errors.New("forked Worker parent runtime differs from Host binding")
	}
	parentOp, err := run.Open(binding.Root, binding.ExperimentID, origin.ParentRun)
	if err != nil {
		return PiRuntimeProfile{}, err
	}
	stored, _, receipt, err := parentOp.ForkReceipt(forked.ForkReceipt.IntentID)
	if err != nil || receipt != forked.ForkReceipt || stored != forked.Forked {
		return PiRuntimeProfile{}, errors.New("forked Worker receipt differs from Host binding")
	}
	experimentOp, err := binding.experiment()
	if err != nil {
		return PiRuntimeProfile{}, err
	}
	bound, err := experimentOp.DecisionBoundEffect(forked.ForkReceipt.IntentID)
	if err != nil || bound.Intent != forked.Forked.Intent || bound.Decision.Action != experiment.DecisionFork || bound.Decision.WorkerRun != origin.ParentRun {
		return PiRuntimeProfile{}, errors.New("forked Worker receipt has no decision-bound fork")
	}
	decisionAt, err := experimentOp.DecisionBoundEffectTime(forked.ForkReceipt.IntentID)
	forkAt, forkErr := parentOp.SessionForkedTime(forked.Forked.ChildSession)
	if err != nil || forkErr != nil || decisionAt.After(forkAt) {
		return PiRuntimeProfile{}, errors.New("forked Worker receipt precedes its decision-bound fork")
	}
	reconciled, err := piadapter.ReconcileForkedSession(parentOp, forked.ForkReceipt.IntentID, piadapter.ForkSpec{
		SDKRoot: parent.Identity.SDKRoot, ContextFilterPath: parent.Identity.ContextFilterPath,
		ParentSession: parent.SessionPath, ChildSessionDir: template.WorkerLaunch.Launch.RuntimeRoot,
	}, parent.Identity)
	if err != nil || reconciled.Forked != forked.Forked {
		return PiRuntimeProfile{}, errors.New("forked Worker child session differs from receipt")
	}
	profile := template.freshProfile()
	profile.SessionPath = reconciled.ChildSessionPath
	profile.resumeExistingSession = true
	if profile.Validate() != nil {
		return PiRuntimeProfile{}, errors.New("forked Worker profile is invalid")
	}
	return profile, nil
}
