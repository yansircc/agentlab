package experiment

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/ledger"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/transaction"
)

const RunManifestContract = "agentlab.run-manifest.v2"

type RunInputs struct {
	Harness        artifact.Ref `json:"harness"`
	Trial          artifact.Ref `json:"trial"`
	Candidate      artifact.Ref `json:"candidate"`
	Adapter        artifact.Ref `json:"adapter"`
	OracleSet      artifact.Ref `json:"oracle_set"`
	Fixture        artifact.Ref `json:"fixture"`
	FixtureReset   artifact.Ref `json:"fixture_reset"`
	EvidencePolicy artifact.Ref `json:"evidence_policy"`
	StopPolicy     artifact.Ref `json:"stop_policy"`
	WorkerRuntime  artifact.Ref `json:"worker_runtime"`
	Environment    artifact.Ref `json:"environment"`
}

type RunManifest struct {
	Contract       string       `json:"contract"`
	WorkerInput    artifact.Ref `json:"worker_input"`
	SourceSnapshot artifact.Ref `json:"source_snapshot"`
	Origin         RunOrigin    `json:"origin"`
	RunInputs
}

type runBinding struct {
	RunID    string       `json:"run_id"`
	Manifest artifact.Ref `json:"manifest"`
}

func (o *Operation) BindRun(runID string, origin RunOrigin, inputs RunInputs) (artifact.Ref, error) {
	return o.bindRun(runID, origin, inputs, nil)
}

func (o *Operation) bindRun(runID string, origin RunOrigin, inputs RunInputs, decision *SupervisorDecision) (artifact.Ref, error) {
	if !idPattern.MatchString(runID) {
		return artifact.Ref{}, errors.New("invalid run id")
	}
	runDir := filepath.Join(o.dir, "runs", runID)
	records, err := ledger.Open(filepath.Join(runDir, "events.jsonl")).Read(0, 1)
	if err != nil {
		return artifact.Ref{}, err
	}
	if len(records) != 0 {
		return artifact.Ref{}, errors.New("run manifest cannot be bound after run events")
	}
	for _, ref := range inputRefs(inputs) {
		if !validRef(ref) {
			return artifact.Ref{}, errors.New("run manifest input is invalid")
		}
		if _, err := o.artifacts.Read(ref); err != nil {
			return artifact.Ref{}, err
		}
	}
	if _, err := source.Load(o.artifacts, inputs.Candidate); err != nil {
		return artifact.Ref{}, errors.New("run manifest candidate is not a source snapshot")
	}
	reset, err := loadFixtureReset(o.artifacts, inputs.FixtureReset)
	if err != nil {
		return artifact.Ref{}, err
	}
	if reset.RunID != runID || reset.Fixture != inputs.Fixture {
		return artifact.Ref{}, errors.New("fixture reset proof does not bind run and fixture")
	}
	current, err := o.current()
	if err != nil {
		return artifact.Ref{}, err
	}
	if current.begun == nil {
		return artifact.Ref{}, ErrNotBegun
	}
	if decision != nil && current.decisions[decision.ID].ID != "" {
		return artifact.Ref{}, errors.New("decision identity already exists")
	}
	if err := o.validateOrigin(runID, origin, current); err != nil {
		return artifact.Ref{}, err
	}
	manifest := RunManifest{Contract: RunManifestContract, WorkerInput: current.begun.WorkerInput, SourceSnapshot: current.begun.Source, Origin: origin, RunInputs: inputs}
	data, err := json.Marshal(manifest)
	if err != nil {
		return artifact.Ref{}, err
	}
	manifestRef, err := o.artifacts.Put(data)
	if err != nil {
		return artifact.Ref{}, err
	}
	binding := runBinding{RunID: runID, Manifest: manifestRef}
	err = o.mutate(func(current *state) error {
		existing := current.runs[runID]
		if decision != nil && current.decisions[decision.ID].ID != "" {
			return errors.New("decision identity already exists")
		}
		if existing.RunID != "" && existing != binding {
			return errors.New("run already has a different manifest")
		}
		if existing.RunID == "" {
			if decision != nil {
				return o.append(eventDecisionRun, decisionRunRecord{Decision: *decision, Binding: binding})
			}
			return o.append(eventRunBound, binding)
		}
		return nil
	})
	if err != nil {
		return artifact.Ref{}, err
	}
	projection, err := json.Marshal(binding)
	if err != nil {
		return artifact.Ref{}, err
	}
	path := filepath.Join(runDir, "manifest.json")
	if err := transaction.WriteOnce(path, projection, 0o600); err != nil {
		return artifact.Ref{}, err
	}
	return manifestRef, nil
}

func inputRefs(inputs RunInputs) []artifact.Ref {
	return []artifact.Ref{inputs.Harness, inputs.Trial, inputs.Candidate, inputs.Adapter, inputs.OracleSet, inputs.Fixture, inputs.FixtureReset, inputs.EvidencePolicy, inputs.StopPolicy, inputs.WorkerRuntime, inputs.Environment}
}

func (o *Operation) RunManifest(runID string) (RunManifest, artifact.Ref, error) {
	current, err := o.current()
	if err != nil {
		return RunManifest{}, artifact.Ref{}, err
	}
	binding := current.runs[runID]
	if binding.RunID == "" {
		return RunManifest{}, artifact.Ref{}, errors.New("run manifest does not exist")
	}
	manifest, err := o.readManifest(binding.Manifest)
	if err != nil {
		return RunManifest{}, artifact.Ref{}, errors.New("run manifest artifact is invalid")
	}
	return manifest, binding.Manifest, nil
}

func validManifest(manifest RunManifest) bool {
	if !validRef(manifest.WorkerInput) || !validRef(manifest.SourceSnapshot) || !manifest.Origin.valid() {
		return false
	}
	for _, ref := range inputRefs(manifest.RunInputs) {
		if !validRef(ref) {
			return false
		}
	}
	return true
}
