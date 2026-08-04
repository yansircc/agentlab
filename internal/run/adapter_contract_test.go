package run

import (
	"testing"
	"time"
)

func TestAttachedAdapterCapabilityDowngradeFailsBeforeStart(t *testing.T) {
	required := RequiredAdapterCapabilities()
	variants := []AdapterCapabilities{
		{PublicEventsOnly: true, ThinkingExclusion: true, ToolCorrelation: true},
		{DurableCursor: true, ThinkingExclusion: true, ToolCorrelation: true},
		{DurableCursor: true, PublicEventsOnly: true, ToolCorrelation: true},
		{DurableCursor: true, PublicEventsOnly: true, ThinkingExclusion: true},
	}
	for index, capabilities := range variants {
		op, _ := Open(t.TempDir(), "experiment", "downgrade")
		bindTestManifest(t, op)
		policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
		_, err := op.BeginAttached(AttachedSpec{
			Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: capabilities,
		})
		if err == nil {
			t.Fatalf("capability downgrade %d from %#v was accepted", index, required)
		}
		if records, replayErr := op.Inspect(0, 1); replayErr != nil || len(records) != 0 {
			t.Fatalf("downgrade %d mutated run: %#v, %v", index, records, replayErr)
		}
	}
}

func TestAdapterRejectsKindsOutsideCoreEvidenceAlgebra(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "kind")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	_, err := op.BeginAttached(AttachedSpec{
		Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities(),
	})
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := op.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Commit([]byte("next"), AdapterBatch{Events: []AdapterEvent{{Kind: "pi_private_kind", Label: "bad", Raw: []byte("bad")}}})
	_ = writer.Close()
	if err == nil {
		t.Fatal("adapter-specific core evidence kind was accepted")
	}
}

func TestAdapterSourceLocatorIsWriteOnceAndResolvesOneEvidenceRef(t *testing.T) {
	op, _ := Open(t.TempDir(), "experiment", "source-locator")
	bindTestManifest(t, op)
	policy := StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := op.BeginAttached(AttachedSpec{Adapter: "test", StreamID: "stream", InitialCursor: []byte("cursor"), Policy: policy, Capabilities: RequiredAdapterCapabilities()}); err != nil {
		t.Fatal(err)
	}
	source := "sha256:public-entry"
	writer, _, err := op.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit([]byte("cursor-1"), AdapterBatch{Events: []AdapterEvent{{Kind: EvidenceAssistantMessage, SourceLocator: source, Label: "public", Raw: []byte("public")}}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	ref, err := op.EvidenceForSourceLocator(source)
	if err != nil || ref != (EvidenceRef{ExperimentID: "experiment", RunID: "source-locator", Sequence: 2, Item: 0}) {
		t.Fatalf("source evidence ref = %#v, %v", ref, err)
	}
	writer, _, err = op.AcquireAdapterWriter("test")
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err := writer.Commit([]byte("cursor-2"), AdapterBatch{Events: []AdapterEvent{{Kind: EvidenceAssistantMessage, SourceLocator: source, Label: "duplicate", Raw: []byte("duplicate")}}}); err == nil {
		t.Fatal("duplicate adapter source locator was accepted")
	}
}
