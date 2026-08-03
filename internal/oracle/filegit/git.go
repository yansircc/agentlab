package filegit

import (
	"errors"
	"os"
	"os/exec"

	"github.com/yansircc/agentlab/internal/artifact"
)

type boundedWriter struct {
	data      []byte
	limit     int
	truncated bool
}

func (w *boundedWriter) Write(input []byte) (int, error) {
	remaining := w.limit - len(w.data)
	if remaining > 0 {
		w.data = append(w.data, input[:min(remaining, len(input))]...)
	}
	if len(input) > remaining {
		w.truncated = true
	}
	return len(input), nil
}

func captureGit(store artifact.Store, spec Spec) (GitFact, error) {
	bytes, err := os.ReadFile(spec.GitExecutable)
	if err != nil {
		return GitFact{}, err
	}
	executable, err := store.Put(bytes)
	if err != nil {
		return GitFact{}, err
	}
	stdout, stderr := &boundedWriter{limit: spec.MaxGitBytes}, &boundedWriter{limit: spec.MaxGitBytes}
	cmd := exec.Command(spec.GitExecutable, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = spec.Root
	cmd.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	status, err := store.Put(stdout.data)
	if err != nil {
		return GitFact{}, err
	}
	fact := GitFact{Executable: executable, Status: status, ExitCode: gitExitCode(runErr), Truncated: stdout.truncated || stderr.truncated}
	if runErr != nil {
		fact.Failure = runErr.Error() + ": " + string(stderr.data)
	}
	return fact, nil
}

func gitExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
