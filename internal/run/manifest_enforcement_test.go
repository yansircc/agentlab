package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOwnedStartRequiresManifestBeforeSpawn(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "spawned")
	operation, _ := Open(root, "experiment", "missing-manifest")
	policy := StopPolicy{
		FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second,
		HardIdleTimeout: 3 * time.Second, OwnsWorkerProcess: true,
	}
	_, err := operation.Start(context.Background(), "missing-manifest", StartSpec{PublicCommand: []string{"/bin/sh", "-c", "touch " + marker}, Policy: policy})
	if err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("missing manifest error = %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("worker spawned before manifest check: %v", err)
	}
	if _, err := os.Stat(filepath.Join(operation.dir, "launch-attempts")); !os.IsNotExist(err) {
		t.Fatalf("launch attempt created before manifest check: %v", err)
	}
}

func TestAttachedStartRequiresManifestBeforeReceipt(t *testing.T) {
	root := t.TempDir()
	operation, _ := Open(root, "experiment", "missing-attached-manifest")
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	_, err := operation.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()})
	if err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("missing manifest error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(operation.dir, "request.json")); !os.IsNotExist(err) {
		t.Fatalf("attached receipt created before manifest check: %v", err)
	}
}
