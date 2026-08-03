package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/oracle"
)

func Execute(ctx context.Context, store artifact.Store, spec Spec) (Result, error) {
	if err := validateSpec(spec); err != nil {
		return Result{}, err
	}
	secrets, err := resolveSecrets(spec.SecretEnvironmentHandles)
	if err != nil {
		return Result{}, err
	}
	executableBytes, err := os.ReadFile(spec.Command[0])
	if err != nil {
		return Result{}, err
	}
	executable, err := store.Put(executableBytes)
	if err != nil {
		return Result{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runContext, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.Directory
	cmd.Env = buildEnvironment(spec.PublicEnvironment, secrets)
	captureLimit := spec.MaxOutputBytes + longestSecret(secrets)
	stdout, stderr := newBoundedBuffer(captureLimit), newBoundedBuffer(captureLimit)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	stdout.data = redact(stdout.data, secrets)
	stderr.data = redact(stderr.data, secrets)
	stdout.data, stdout.truncated = capOutput(stdout.data, spec.MaxOutputBytes, stdout.truncated)
	stderr.data, stderr.truncated = capOutput(stderr.data, spec.MaxOutputBytes, stderr.truncated)
	stdoutRef, err := store.Put(stdout.data)
	if err != nil {
		return Result{}, err
	}
	stderrRef, err := store.Put(stderr.data)
	if err != nil {
		return Result{}, err
	}
	output := Output{Executable: executable, Stdout: stdoutRef, Stderr: stderrRef, ExitCode: exitCode(runErr), Truncated: stdout.truncated || stderr.truncated}
	if runErr != nil {
		output.Failure = runErr.Error()
	}
	receipt, err := oracle.Record(store, "command", spec, output, spec.SideEffects)
	if err != nil {
		return Result{}, err
	}
	return Result{Receipt: receipt, Output: output}, nil
}

func validateSpec(spec Spec) error {
	if len(spec.Command) == 0 || !filepath.IsAbs(spec.Command[0]) || !filepath.IsAbs(spec.Directory) {
		return errors.New("absolute executable and directory are required")
	}
	if spec.Timeout <= 0 || spec.MaxOutputBytes < 1 || spec.MaxOutputBytes > 8*1024*1024 || len(spec.SideEffects) == 0 {
		return errors.New("timeout, bounded output, and side effects are required")
	}
	for key := range spec.PublicEnvironment {
		if key == "" || strings.ContainsRune(key, '=') {
			return errors.New("public environment key is invalid")
		}
		if _, duplicated := spec.SecretEnvironmentHandles[key]; duplicated {
			return errors.New("public and secret environment keys overlap")
		}
	}
	return nil
}

func resolveSecrets(handles map[string]string) (map[string]string, error) {
	values := make(map[string]string, len(handles))
	for variable, handle := range handles {
		if variable == "" || strings.ContainsRune(variable, '=') || handle == "" {
			return nil, errors.New("secret environment mapping is invalid")
		}
		value, ok := os.LookupEnv(handle)
		if !ok {
			return nil, errors.New("secret handle is unavailable: " + handle)
		}
		if len(value) > 64*1024 {
			return nil, errors.New("secret handle exceeds redaction bound: " + handle)
		}
		values[variable] = value
	}
	return values, nil
}

func longestSecret(secrets map[string]string) int {
	longest := 0
	for _, value := range secrets {
		longest = max(longest, len(value))
	}
	return longest
}

func capOutput(data []byte, limit int, already bool) ([]byte, bool) {
	if len(data) <= limit {
		return data, already
	}
	return data[:limit], true
}

func buildEnvironment(public, secret map[string]string) []string {
	merged := make(map[string]string, len(public)+len(secret))
	for key, value := range public {
		merged[key] = value
	}
	for key, value := range secret {
		merged[key] = value
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+merged[key])
	}
	return result
}

func redact(data []byte, secrets map[string]string) []byte {
	type secret struct{ variable, value string }
	ordered := make([]secret, 0, len(secrets))
	for variable, value := range secrets {
		ordered = append(ordered, secret{variable: variable, value: value})
	}
	sort.Slice(ordered, func(left, right int) bool {
		if len(ordered[left].value) != len(ordered[right].value) {
			return len(ordered[left].value) > len(ordered[right].value)
		}
		return ordered[left].variable < ordered[right].variable
	})
	for _, secret := range ordered {
		variable, value := secret.variable, secret.value
		if value == "" {
			continue
		}
		replacement := []byte("[REDACTED:" + variable + "]")
		data = bytes.ReplaceAll(data, []byte(value), replacement)
	}
	return data
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
