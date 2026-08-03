package run

import (
	"encoding/json"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/transaction"
)

func bindTestManifest(t *testing.T, operation *Operation) artifact.Ref {
	t.Helper()
	manifest, err := operation.artifacts.Put([]byte(`{"contract":"test-run-manifest"}`))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(manifestReceipt{RunID: operation.runID, Manifest: manifest})
	if err := transaction.WriteOnce(operation.dir+"/manifest.json", data, 0o600); err != nil {
		t.Fatal(err)
	}
	return manifest
}
