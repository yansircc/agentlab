package experiment

import "github.com/yansircc/agentlab/internal/artifact"

func validRef(ref artifact.Ref) bool {
	return ref.Valid()
}
