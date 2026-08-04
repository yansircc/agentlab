package run

import (
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestHostOracleEvidenceIsOneActiveRunFact(t *testing.T) {
	op, err := Open(t.TempDir(), "oracle-experiment", "worker")
	if err != nil {
		t.Fatal(err)
	}
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "worker", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	raw, err := op.artifacts.Put([]byte("host-owned oracle artifact"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := op.RecordHostOracleEvidence(raw)
	if err != nil {
		t.Fatal(err)
	}
	item, err := op.EvidenceAt(ref)
	if err != nil || item.Kind != EvidenceOracle || item.Label != "host_objective_oracle" || item.Raw != raw {
		t.Fatalf("host oracle evidence = %#v, %v", item, err)
	}
	if _, err := op.RecordHostOracleEvidence(raw); err == nil {
		t.Fatal("run accepted a second Host oracle artifact")
	}
	values, err := op.OracleEvidence()
	if err != nil || len(values) != 1 || values[0].Raw != raw {
		t.Fatalf("oracle projection = %#v, %v", values, err)
	}
	if _, err := op.RecordHostOracleEvidence(artifact.Ref{}); err == nil {
		t.Fatal("run accepted an invalid Host oracle reference")
	}
}
