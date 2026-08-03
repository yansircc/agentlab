package experiment

import "github.com/yansircc/agentlab/internal/artifact"

func validRef(ref artifact.Ref) bool {
	return ref.Algorithm == "sha256" && len(ref.Digest) == 64 && ref.Size >= 0
}
