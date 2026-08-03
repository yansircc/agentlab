package experiment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
)

const (
	freshOriginKind  = "fresh"
	spliceOriginKind = "splice"
)

type FreshOrigin struct{}

type SpliceOrigin struct{ spec SpliceOriginSpec }

type SpliceOriginSpec struct {
	ParentRun         string            `json:"parent_run"`
	ParentEvidence    run.EvidenceRef   `json:"parent_evidence"`
	RuntimeCheckpoint artifact.Ref      `json:"runtime_checkpoint"`
	PublicPrefix      artifact.Ref      `json:"public_prefix"`
	Intervention      *artifact.Ref     `json:"intervention,omitempty"`
	ReasonEvidence    []run.EvidenceRef `json:"reason_evidence"`
}

type originVariant interface{ originKind() string }

func (FreshOrigin) originKind() string  { return freshOriginKind }
func (SpliceOrigin) originKind() string { return spliceOriginKind }

// RunOrigin is closed: constructors are the only way to form a legal origin.
type RunOrigin struct{ variant originVariant }

func NewFreshOrigin() RunOrigin { return RunOrigin{variant: FreshOrigin{}} }

func NewSpliceOrigin(spec SpliceOriginSpec) (RunOrigin, error) {
	if err := spec.Validate(); err != nil {
		return RunOrigin{}, err
	}
	return RunOrigin{variant: SpliceOrigin{spec: cloneSpliceSpec(spec)}}, nil
}

func (o RunOrigin) IsFresh() bool {
	_, ok := o.variant.(FreshOrigin)
	return ok
}

func (o RunOrigin) Splice() (SpliceOriginSpec, bool) {
	value, ok := o.variant.(SpliceOrigin)
	if !ok {
		return SpliceOriginSpec{}, false
	}
	return cloneSpliceSpec(value.spec), true
}

func (o RunOrigin) valid() bool {
	if o.IsFresh() {
		return true
	}
	value, ok := o.Splice()
	return ok && value.Validate() == nil
}

func (s SpliceOriginSpec) Validate() error {
	if !idPattern.MatchString(s.ParentRun) || s.ParentEvidence.RunID != s.ParentRun || s.ParentEvidence.ExperimentID == "" || s.ParentEvidence.Sequence == 0 || s.ParentEvidence.Item < 0 {
		return errors.New("splice parent evidence is invalid")
	}
	if !validRef(s.RuntimeCheckpoint) || !validRef(s.PublicPrefix) || len(s.ReasonEvidence) == 0 || len(s.ReasonEvidence) > 50 {
		return errors.New("splice checkpoint, prefix, and reason evidence are required")
	}
	if s.Intervention != nil && !validRef(*s.Intervention) {
		return errors.New("splice intervention is invalid")
	}
	seen := map[run.EvidenceRef]bool{}
	for _, ref := range s.ReasonEvidence {
		if ref.ExperimentID == "" || ref.RunID != s.ParentRun || ref.Sequence == 0 || ref.Item < 0 || seen[ref] {
			return errors.New("splice reason evidence is invalid")
		}
		seen[ref] = true
	}
	return nil
}

func (o RunOrigin) MarshalJSON() ([]byte, error) {
	if o.IsFresh() {
		return json.Marshal(originWire{Kind: freshOriginKind})
	}
	spec, ok := o.Splice()
	if !ok || spec.Validate() != nil {
		return nil, errors.New("invalid run origin")
	}
	return json.Marshal(originWire{
		Kind: spliceOriginKind, ParentRun: spec.ParentRun, ParentEvidence: &spec.ParentEvidence,
		RuntimeCheckpoint: &spec.RuntimeCheckpoint, PublicPrefix: &spec.PublicPrefix,
		Intervention: spec.Intervention, ReasonEvidence: spec.ReasonEvidence,
	})
}

func (o *RunOrigin) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeOrigin(data, &fields); err != nil {
		return err
	}
	var value originWire
	if err := decodeOrigin(data, &value); err != nil {
		return err
	}
	switch value.Kind {
	case freshOriginKind:
		if len(fields) != 1 {
			return errors.New("fresh origin carries splice fields")
		}
		*o = NewFreshOrigin()
		return nil
	case spliceOriginKind:
		if !validSpliceWire(fields) || value.ParentEvidence == nil || value.RuntimeCheckpoint == nil || value.PublicPrefix == nil {
			return errors.New("splice origin fields are invalid")
		}
		origin, err := NewSpliceOrigin(SpliceOriginSpec{
			ParentRun: value.ParentRun, ParentEvidence: *value.ParentEvidence,
			RuntimeCheckpoint: *value.RuntimeCheckpoint, PublicPrefix: *value.PublicPrefix,
			Intervention: value.Intervention, ReasonEvidence: value.ReasonEvidence,
		})
		if err != nil {
			return err
		}
		*o = origin
		return nil
	default:
		return fmt.Errorf("unknown run origin kind %q", value.Kind)
	}
}

type originWire struct {
	Kind              string            `json:"kind"`
	ParentRun         string            `json:"parent_run,omitempty"`
	ParentEvidence    *run.EvidenceRef  `json:"parent_evidence,omitempty"`
	RuntimeCheckpoint *artifact.Ref     `json:"runtime_checkpoint,omitempty"`
	PublicPrefix      *artifact.Ref     `json:"public_prefix,omitempty"`
	Intervention      *artifact.Ref     `json:"intervention,omitempty"`
	ReasonEvidence    []run.EvidenceRef `json:"reason_evidence,omitempty"`
}

func validSpliceWire(fields map[string]json.RawMessage) bool {
	required := map[string]bool{"kind": true, "parent_run": true, "parent_evidence": true, "runtime_checkpoint": true, "public_prefix": true, "reason_evidence": true}
	for key := range fields {
		if key != "intervention" && !required[key] {
			return false
		}
		delete(required, key)
	}
	return len(required) == 0
}

func decodeOrigin(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("origin has trailing input")
	}
	return nil
}

func cloneSpliceSpec(value SpliceOriginSpec) SpliceOriginSpec {
	value.ReasonEvidence = append([]run.EvidenceRef(nil), value.ReasonEvidence...)
	if value.Intervention != nil {
		copy := *value.Intervention
		value.Intervention = &copy
	}
	return value
}
