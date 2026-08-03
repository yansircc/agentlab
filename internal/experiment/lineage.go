package experiment

import (
	"errors"
	"fmt"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

func (o *Operation) readManifest(ref artifact.Ref) (RunManifest, error) {
	data, err := o.artifacts.Read(ref)
	if err != nil {
		return RunManifest{}, err
	}
	var manifest RunManifest
	if err := decode(data, &manifest); err != nil || manifest.Contract != RunManifestContract || !validManifest(manifest) {
		return RunManifest{}, errors.New("run manifest artifact is invalid")
	}
	return manifest, nil
}

func (o *Operation) validateRunLineage(current state) error {
	parents := map[string]string{}
	for _, runID := range current.runOrder {
		binding := current.runs[runID]
		manifest, err := o.readManifest(binding.Manifest)
		if err != nil {
			return err
		}
		if _, err := source.Load(o.artifacts, manifest.Candidate); err != nil {
			return errors.New("run manifest candidate is not a source snapshot")
		}
		if origin, ok := manifest.Origin.Splice(); ok {
			parents[runID] = origin.ParentRun
		}
	}
	if err := validateAcyclicOrigins(current.runOrder, parents); err != nil {
		return err
	}
	for _, runID := range current.runOrder {
		manifest, err := o.readManifest(current.runs[runID].Manifest)
		if err != nil {
			return err
		}
		if origin, ok := manifest.Origin.Splice(); ok {
			if err := o.validateSpliceOrigin(runID, origin, current); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAcyclicOrigins(runOrder []string, parents map[string]string) error {
	seen, active := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(runID string) error {
		if active[runID] {
			return fmt.Errorf("run origin cycle includes %q", runID)
		}
		if seen[runID] {
			return nil
		}
		seen[runID], active[runID] = true, true
		if parent := parents[runID]; parent != "" {
			if err := visit(parent); err != nil {
				return err
			}
		}
		delete(active, runID)
		return nil
	}
	for _, runID := range runOrder {
		if err := visit(runID); err != nil {
			return err
		}
	}
	return nil
}

func (o *Operation) validateOrigin(runID string, origin RunOrigin, current state) error {
	if !origin.valid() {
		return errors.New("run origin is invalid")
	}
	splice, ok := origin.Splice()
	if !ok {
		return nil
	}
	return o.validateSpliceOrigin(runID, splice, current)
}

func (o *Operation) validateSpliceOrigin(runID string, value SpliceOriginSpec, current state) error {
	if value.ParentRun == runID || value.ParentEvidence.ExperimentID != o.id || value.ParentEvidence.RunID != value.ParentRun {
		return errors.New("splice parent is invalid")
	}
	if current.runs[value.ParentRun].RunID == "" {
		return errors.New("splice parent run does not exist")
	}
	for _, ref := range value.ReasonEvidence {
		if ref.ExperimentID != o.id || ref.RunID != value.ParentRun {
			return errors.New("splice reason evidence belongs to another experiment or run")
		}
	}
	for _, ref := range spliceArtifacts(value) {
		if _, err := o.artifacts.Read(ref); err != nil {
			return fmt.Errorf("splice artifact is unavailable: %w", err)
		}
	}
	parent, err := run.Open(o.root, o.id, value.ParentRun)
	if err != nil {
		return err
	}
	if _, err := parent.EvidenceAt(value.ParentEvidence); err != nil {
		return fmt.Errorf("splice parent evidence is unavailable: %w", err)
	}
	for _, ref := range value.ReasonEvidence {
		if _, err := parent.EvidenceAt(ref); err != nil {
			return fmt.Errorf("splice reason evidence is unavailable: %w", err)
		}
	}
	prefix, err := parent.RuntimeCheckpointPublicPrefix(value.RuntimeCheckpoint)
	if err != nil || prefix != value.PublicPrefix {
		return errors.New("splice checkpoint does not belong to parent run")
	}
	return nil
}

func spliceArtifacts(value SpliceOriginSpec) []artifact.Ref {
	refs := []artifact.Ref{value.RuntimeCheckpoint, value.PublicPrefix}
	if value.Intervention != nil {
		refs = append(refs, *value.Intervention)
	}
	return refs
}

// Lineage is a replay-derived execution graph; it is never persisted separately.
type Lineage struct {
	Roots []string      `json:"roots"`
	Edges []LineageEdge `json:"edges"`
}

type LineageEdge struct {
	ParentRun       string            `json:"parent_run"`
	ChildRun        string            `json:"child_run"`
	ParentCandidate artifact.Ref      `json:"parent_candidate"`
	ChildCandidate  artifact.Ref      `json:"child_candidate"`
	ParentEvidence  run.EvidenceRef   `json:"parent_evidence"`
	Checkpoint      artifact.Ref      `json:"checkpoint"`
	PublicPrefix    artifact.Ref      `json:"public_prefix"`
	Intervention    *artifact.Ref     `json:"intervention,omitempty"`
	ReasonEvidence  []run.EvidenceRef `json:"reason_evidence"`
}

func (o *Operation) Lineage() (Lineage, error) {
	current, err := o.current()
	if err != nil {
		return Lineage{}, err
	}
	manifests := map[string]RunManifest{}
	for _, runID := range current.runOrder {
		manifest, err := o.readManifest(current.runs[runID].Manifest)
		if err != nil {
			return Lineage{}, err
		}
		manifests[runID] = manifest
	}
	result := Lineage{}
	for _, runID := range current.runOrder {
		child := manifests[runID]
		origin, ok := child.Origin.Splice()
		if !ok {
			result.Roots = append(result.Roots, runID)
			continue
		}
		parent := manifests[origin.ParentRun]
		edge := LineageEdge{
			ParentRun: origin.ParentRun, ChildRun: runID, ParentCandidate: parent.Candidate, ChildCandidate: child.Candidate,
			ParentEvidence: origin.ParentEvidence, Checkpoint: origin.RuntimeCheckpoint, PublicPrefix: origin.PublicPrefix,
			ReasonEvidence: append([]run.EvidenceRef(nil), origin.ReasonEvidence...),
		}
		if origin.Intervention != nil {
			copy := *origin.Intervention
			edge.Intervention = &copy
		}
		result.Edges = append(result.Edges, edge)
	}
	return result, nil
}
