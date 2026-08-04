package comparison

import (
	"reflect"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestOracleEvidenceRequiresCanonicalCompleteBinding(t *testing.T) {
	root := t.TempDir()
	store := artifact.NewStore(root)
	value := OracleEvidence{
		Contract: OracleEvidenceContract, RunID: "worker-1", Candidate: testRef("candidate"), Trial: testRef("trial"), OracleSet: testRef("oracles"),
		Claims: []OracleClaim{{ID: "target-owner", Passed: true, HeldOut: true}},
	}
	data, err := EncodeOracleEvidence(value)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := store.Put(data)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadOracleEvidence(store, ref)
	if err != nil || !reflect.DeepEqual(loaded, value) {
		t.Fatalf("oracle evidence = %#v, %v", loaded, err)
	}
	invalid, err := store.Put([]byte(`{"contract":"agentlab.oracle-evidence.v1","run_id":"worker-1","candidate":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOracleEvidence(store, invalid); err == nil {
		t.Fatal("incomplete oracle evidence was accepted")
	}
	nonCanonical, err := store.Put(append(append([]byte(nil), data...), '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOracleEvidence(store, nonCanonical); err == nil {
		t.Fatal("non-canonical oracle evidence was accepted")
	}
}
