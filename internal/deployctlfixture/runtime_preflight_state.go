package deployctlfixture

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/metaaudit"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/strictjson"
	"github.com/yansircc/agentlab/internal/transaction"
)

const runtimePreflightLocatorContract = "agentlab.deployctl-runtime-preflight.v1"

type runtimePreflightLocator struct {
	Contract      string `json:"contract"`
	EvaluatedRoot string `json:"evaluated_root"`
	AuditRoot     string `json:"audit_root"`
}

// LoadRuntimePreflight reopens a verified Host assembly without copying
// evaluated facts. The only durable Host state is the two capability roots;
// preparation, manifest, audit and runtime artifacts remain their own owners.
func LoadRuntimePreflight(hostRoot string) (Preflight, error) {
	locator, err := readRuntimePreflightLocator(hostRoot)
	if err != nil {
		return Preflight{}, err
	}
	fixture, err := OpenFixture(filepath.Join(locator.EvaluatedRoot, "fixture"))
	if err != nil {
		return Preflight{}, err
	}
	prep, err := preparation.Open(locator.EvaluatedRoot, preparationID)
	if err != nil {
		return Preflight{}, err
	}
	prepared, err := prep.Status()
	if err != nil || prepared.Phase != preparation.PhaseSealed {
		return Preflight{}, errors.New("deployctl preparation is unavailable")
	}
	experimentOp, err := experiment.Open(locator.EvaluatedRoot, experimentID)
	if err != nil {
		return Preflight{}, err
	}
	manifest, _, err := experimentOp.RunManifest(baselineRunID)
	if err != nil || !manifest.Origin.IsFresh() {
		return Preflight{}, errors.New("deployctl baseline manifest is unavailable")
	}
	store := artifact.NewStore(filepath.Join(locator.EvaluatedRoot, "artifacts"))
	binding, err := loadRuntimeBinding(store, manifest.WorkerRuntime)
	if err != nil {
		return Preflight{}, err
	}
	audit, err := metaaudit.Open(locator.AuditRoot, auditID)
	if err != nil {
		return Preflight{}, err
	}
	auditStatus, err := audit.Status()
	if err != nil || auditStatus.Trial.ExperimentID != experimentID {
		return Preflight{}, errors.New("deployctl audit preflight is unavailable")
	}
	value := Preflight{
		EvaluatedRoot: locator.EvaluatedRoot, AuditRoot: locator.AuditRoot, PreparationID: preparationID, ExperimentID: experimentID, BaselineRunID: baselineRunID, AuditID: auditID,
		Fixture: fixture, WorkerInput: prepared.WorkerInput, SourceSnapshot: prepared.Source, Candidate: manifest.Candidate, CandidateExecutable: binding.CandidateExecutable,
		FixtureReset: manifest.FixtureReset, LiveCanary: manifest.Adapter, Inputs: manifest.RunInputs, GroundTruth: auditStatus.Trial.GroundTruth,
		hostRoot: hostRoot, runtimePlanPath: filepath.Join(hostRoot, "pi-runtime-plan.json"),
	}
	if err := value.verifyRecordedRoots(); err != nil {
		return Preflight{}, err
	}
	return value, value.verifyRuntime()
}

func writeRuntimePreflightLocator(hostRoot, evaluatedRoot, auditRoot string) error {
	if !validRuntimePreflightRoots(hostRoot, evaluatedRoot, auditRoot) {
		return errors.New("deployctl runtime preflight locator is invalid")
	}
	data, err := json.Marshal(runtimePreflightLocator{Contract: runtimePreflightLocatorContract, EvaluatedRoot: evaluatedRoot, AuditRoot: auditRoot})
	if err != nil {
		return err
	}
	return transaction.WriteOnce(filepath.Join(hostRoot, "preflight.json"), data, 0o600)
}

func readRuntimePreflightLocator(hostRoot string) (runtimePreflightLocator, error) {
	info, err := os.Lstat(hostRoot)
	if !filepath.IsAbs(hostRoot) || err != nil || !info.IsDir() {
		return runtimePreflightLocator{}, errors.New("deployctl runtime host root is invalid")
	}
	data, err := os.ReadFile(filepath.Join(hostRoot, "preflight.json"))
	if err != nil {
		return runtimePreflightLocator{}, err
	}
	var value runtimePreflightLocator
	if strictjson.Decode(data, &value) != nil || value.Contract != runtimePreflightLocatorContract || !validRuntimePreflightRoots(hostRoot, value.EvaluatedRoot, value.AuditRoot) {
		return runtimePreflightLocator{}, errors.New("deployctl runtime preflight locator is invalid")
	}
	return value, nil
}

func validRuntimePreflightRoots(hostRoot, evaluatedRoot, auditRoot string) bool {
	return disjointRoots(hostRoot, evaluatedRoot) && disjointRoots(hostRoot, auditRoot) && disjointRoots(evaluatedRoot, auditRoot)
}
