package deployctlfixture

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/source"
)

// BuildCandidate compiles only a sealed source snapshot into an executable
// beneath a new Host-owned workspace. The receipt binds source and bytes.
func BuildCandidate(store artifact.Store, candidate artifact.Ref, workspace, executable string) (artifact.Ref, error) {
	if !candidate.Valid() || !newWorkspace(workspace) || !insideWorkspace(workspace, executable) {
		return artifact.Ref{}, errors.New("deployctl build paths are invalid")
	}
	snapshot, err := source.Load(store, candidate)
	if err != nil || materializeSource(store, snapshot, workspace) != nil {
		return artifact.Ref{}, errors.New("deployctl candidate snapshot is invalid")
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		return artifact.Ref{}, err
	}
	if err := os.Mkdir(filepath.Dir(executable), 0o700); err != nil {
		return artifact.Ref{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, goPath, "build", "-trimpath", "-o", executable, ".")
	command.Dir, command.Env = workspace, os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		return artifact.Ref{}, errors.New("deployctl candidate did not build: " + strings.TrimSpace(string(output)))
	}
	binary, err := os.ReadFile(executable)
	if err != nil || len(binary) == 0 {
		return artifact.Ref{}, errors.New("deployctl executable is invalid")
	}
	ref, err := store.Put(binary)
	if err != nil {
		return artifact.Ref{}, err
	}
	return run.BindCandidateExecutable(store, candidate, ref)
}

func VerifyBuild(store artifact.Store, receipt, candidate artifact.Ref, executable string) error {
	return run.VerifyCandidateExecutable(store, receipt, candidate, executable)
}

func newWorkspace(root string) bool {
	return filepath.IsAbs(root) && os.Mkdir(root, 0o700) == nil
}

func insideWorkspace(root, value string) bool {
	if !filepath.IsAbs(value) {
		return false
	}
	relative, err := filepath.Rel(root, value)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func materializeSource(store artifact.Store, snapshot source.Snapshot, root string) error {
	for _, file := range snapshot.Files {
		data, err := store.Read(file.Artifact)
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil || os.WriteFile(path, data, 0o600) != nil {
			return errors.New("deployctl source materialization failed")
		}
	}
	return nil
}
