package run

import (
	"encoding/json"
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
)

// CoderProfile contains only immutable Host-issued refs. Paths, commands, and
// session locators are kept outside the model and ledger capability surface.
type CoderProfile struct {
	Handoff            artifact.Ref `json:"handoff"`
	SourceSnapshot     artifact.Ref `json:"source_snapshot"`
	CandidateWorkspace artifact.Ref `json:"candidate_workspace"`
	CapabilityProfile  artifact.Ref `json:"capability_profile"`
}

func (value CoderProfile) Validate() error {
	for _, ref := range value.refs() {
		if !validRef(ref) {
			return errors.New("coder profile ref is invalid")
		}
	}
	return nil
}

func (value CoderProfile) refs() []artifact.Ref {
	return []artifact.Ref{value.Handoff, value.SourceSnapshot, value.CandidateWorkspace, value.CapabilityProfile}
}

type StartPayload struct {
	Coder *CoderProfile `json:"coder,omitempty"`
}

func EncodeStartPayload(kind effect.Kind, value StartPayload) ([]byte, error) {
	if (kind == effect.WorkerStart && value.Coder != nil) || (kind == effect.CoderStart && (value.Coder == nil || value.Coder.Validate() != nil)) || (kind != effect.WorkerStart && kind != effect.CoderStart) {
		return nil, errors.New("start payload is invalid")
	}
	return json.Marshal(value)
}

func (o *Operation) CoderProfile(intent effect.Intent) (CoderProfile, error) {
	if intent.Kind != effect.CoderStart || intent.RunID != o.runID || intent.Validate() != nil {
		return CoderProfile{}, errors.New("coder start intent is invalid")
	}
	payload, err := o.ReadEffectPayload(intent)
	if err != nil {
		return CoderProfile{}, err
	}
	var value StartPayload
	if strictjson.Decode(payload, &value) != nil || value.Coder == nil {
		return CoderProfile{}, errors.New("coder profile is invalid")
	}
	if _, err := EncodeStartPayload(effect.CoderStart, value); err != nil {
		return CoderProfile{}, err
	}
	for _, ref := range value.Coder.refs() {
		if _, err := o.artifacts.Read(ref); err != nil {
			return CoderProfile{}, err
		}
	}
	return *value.Coder, nil
}

func (o *Operation) CoderHandoff(intent effect.Intent) (artifact.Ref, error) {
	profile, err := o.CoderProfile(intent)
	return profile.Handoff, err
}
