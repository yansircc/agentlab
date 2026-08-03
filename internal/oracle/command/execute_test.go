package command

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestCommandReceiptRedactsHandlesAndBoundsOutput(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	t.Setenv("ORACLE_TEST_SECRET", "must-not-persist")
	result, err := Execute(context.Background(), store, Spec{
		Command:   []string{"/bin/sh", "-c", `printf '%s' "$TOKEN"; printf '123456789012345678901234567890123456789' >&2`},
		Directory: t.TempDir(), Timeout: time.Second, MaxOutputBytes: 32,
		SecretEnvironmentHandles: map[string]string{"TOKEN": "ORACLE_TEST_SECRET"},
		SideEffects:              []string{"none"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, _ := store.Read(result.Output.Stdout)
	stderr, _ := store.Read(result.Output.Stderr)
	config, _ := store.Read(result.Receipt.Configuration)
	if strings.Contains(string(stdout), "must-not-persist") || strings.Contains(string(config), "must-not-persist") {
		t.Fatal("resolved secret persisted")
	}
	if string(stdout) != "[REDACTED:TOKEN]" || string(stderr) != "12345678901234567890123456789012" || !result.Output.Truncated {
		t.Fatalf("stdout=%q stderr=%q output=%#v", stdout, stderr, result.Output)
	}
}

func TestCommandRequiresExactExecutionBoundary(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	if _, err := Execute(context.Background(), store, Spec{Command: []string{"sh"}, Directory: ".", Timeout: time.Second, MaxOutputBytes: 10, SideEffects: []string{"none"}}); err == nil {
		t.Fatal("relative execution boundary was accepted")
	}
	if _, err := Execute(context.Background(), store, Spec{Command: []string{"/bin/sh"}, Directory: t.TempDir(), Timeout: time.Second, MaxOutputBytes: 0, SideEffects: []string{"none"}}); err == nil {
		t.Fatal("unbounded output was accepted")
	}
	if _, err := Execute(context.Background(), store, Spec{
		Command: []string{"/bin/sh"}, Directory: t.TempDir(), Timeout: time.Second, MaxOutputBytes: 10,
		PublicEnvironment: map[string]string{"TOKEN": "public"}, SecretEnvironmentHandles: map[string]string{"TOKEN": "HANDLE"}, SideEffects: []string{"none"},
	}); err == nil {
		t.Fatal("overlapping public and secret environment was accepted")
	}
}

func TestCommandRedactsOverlappingSecretsWithoutSuffixLeak(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	t.Setenv("SHORT_SECRET", "token")
	t.Setenv("LONG_SECRET", "token-suffix")
	result, err := Execute(context.Background(), store, Spec{
		Command: []string{"/bin/sh", "-c", `printf '%s' "$LONG"`}, Directory: t.TempDir(),
		Timeout: time.Second, MaxOutputBytes: 64,
		SecretEnvironmentHandles: map[string]string{"SHORT": "SHORT_SECRET", "LONG": "LONG_SECRET"}, SideEffects: []string{"none"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, _ := store.Read(result.Output.Stdout)
	if string(stdout) != "[REDACTED:LONG]" || strings.Contains(string(stdout), "suffix") {
		t.Fatalf("overlapping secret output = %q", stdout)
	}
}
