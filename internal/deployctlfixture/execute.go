package deployctlfixture

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"time"
)

type CommandResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

func (r CommandResult) TerminalSuccess() bool {
	return r.ExitCode == 0 && len(bytes.TrimSpace(r.Stderr)) == 0 && string(bytes.TrimSpace(r.Stdout)) == `{"contract":"deployctl.result.v1","outcome":"success"}`
}

// Execute invokes the public deployctl CLI against only this fixture root.
func (f Fixture) Execute(executable string, arguments ...string) (CommandResult, error) {
	if !f.valid() || !filepath.IsAbs(executable) {
		return CommandResult{}, errors.New("deployctl command is invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = f.root
	command.Env = []string{"DEPLOYCTL_ROOT=" + f.root}
	stdout, err := command.Output()
	result := CommandResult{Stdout: stdout}
	if failure, ok := err.(*exec.ExitError); ok {
		result.ExitCode, result.Stderr = failure.ExitCode(), failure.Stderr
		return result, nil
	}
	if err != nil {
		return CommandResult{}, err
	}
	result.ExitCode = 0
	return result, nil
}
