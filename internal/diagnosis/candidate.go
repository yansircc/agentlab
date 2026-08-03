package diagnosis

import (
	"errors"

	"github.com/yansircc/agentlab/internal/artifact"
)

type RepairCandidate struct {
	ID          string       `json:"id"`
	DiagnosisID string       `json:"diagnosis_id"`
	Artifact    artifact.Ref `json:"artifact"` // sealed source.Snapshot
}

func (c RepairCandidate) Validate() error {
	if !idPattern.MatchString(c.ID) || !idPattern.MatchString(c.DiagnosisID) || !validRef(c.Artifact) {
		return errors.New("repair candidate identity is invalid")
	}
	return nil
}
