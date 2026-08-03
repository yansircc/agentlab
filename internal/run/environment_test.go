package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOwnedWorkerEnvironmentIsExplicitAndSecretsAreRedacted(t *testing.T) {
	root := t.TempDir()
	op, _ := Open(root, "experiment", "environment")
	bindTestManifest(t, op)
	t.Setenv("AGENTLAB_SECRET_HANDLE", "secret-sentinel-value")
	t.Setenv("PARENT_SECRET", "unbound-parent-sentinel")
	result, err := op.Start(context.Background(), "environment", StartSpec{
		PublicCommand:            []string{os.Args[0], "-test.run=TestHelperProcess", "--", "environment"},
		PublicEnvironment:        map[string]string{"AGENTLAB_HELPER": "1", "PUBLIC_VALUE": "public-value"},
		SecretEnvironmentHandles: map[string]string{"SECRET_VALUE": "AGENTLAB_SECRET_HANDLE"},
		Policy:                   StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true},
	})
	if err != nil || result.Code != 0 {
		t.Fatalf("environment worker = %#v, %v", result, err)
	}
	var sawRedaction bool
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), "secret-sentinel-value") || strings.Contains(string(data), "unbound-parent-sentinel") {
			t.Fatalf("secret persisted in %s", path)
		}
		sawRedaction = sawRedaction || strings.Contains(string(data), "[REDACTED_SECRET]")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRedaction {
		t.Fatal("worker secret output was not represented by a redaction marker")
	}
}

func TestMissingSecretHandleFailsBeforeProcessStart(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "missing-secret")
	bindTestManifest(t, op)
	_, err := op.Start(context.Background(), "missing-secret", StartSpec{
		PublicCommand: []string{"/bin/true"}, SecretEnvironmentHandles: map[string]string{"TOKEN": "ABSENT_AGENTLAB_HANDLE"},
		Policy: StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true},
	})
	if err == nil {
		t.Fatal("missing secret handle was accepted")
	}
	if records, replayErr := op.Inspect(0, 1); replayErr != nil || len(records) != 0 {
		t.Fatalf("missing handle mutated run: %#v, %v", records, replayErr)
	}
}
