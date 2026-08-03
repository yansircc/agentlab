package source

import (
	"errors"
	"path"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
)

const Contract = "agentlab.source-snapshot.v1"

type InputFile struct {
	Path    string
	Content []byte
}

type File struct {
	Path     string       `json:"path"`
	Artifact artifact.Ref `json:"artifact"`
}

type Snapshot struct {
	Contract string `json:"contract"`
	Files    []File `json:"files"`
}

func (snapshot Snapshot) Validate() error {
	if snapshot.Contract != Contract || len(snapshot.Files) == 0 || len(snapshot.Files) > 10000 {
		return errors.New("source snapshot contract or file count is invalid")
	}
	previous := ""
	for _, file := range snapshot.Files {
		if !validPath(file.Path) || !validRef(file.Artifact) || (previous != "" && file.Path <= previous) {
			return errors.New("source snapshot file is invalid, duplicated, or unsorted")
		}
		previous = file.Path
	}
	return nil
}

func validPath(value string) bool {
	clean := path.Clean(value)
	return value != "" && len(value) <= 4096 && value == clean && !path.IsAbs(value) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(value, `\`)
}

func validRef(ref artifact.Ref) bool {
	return ref.Valid()
}
