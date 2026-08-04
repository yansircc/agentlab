package diagnosis

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

type RepairCandidate struct {
	ID          string       `json:"id"`
	DiagnosisID string       `json:"diagnosis_id"`
	CoderRun    string       `json:"coder_run"`
	Completion  artifact.Ref `json:"completion"`
	Artifact    artifact.Ref `json:"artifact"` // derived sealed source.Snapshot
}

func (c RepairCandidate) Validate() error {
	if !idPattern.MatchString(c.ID) || !idPattern.MatchString(c.DiagnosisID) || !idPattern.MatchString(c.CoderRun) || !validRef(c.Completion) || !validRef(c.Artifact) {
		return errors.New("repair candidate identity is invalid")
	}
	return nil
}
