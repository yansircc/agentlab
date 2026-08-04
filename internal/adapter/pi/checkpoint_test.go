package pi

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

func TestCheckpointBindsPublicPrefixToAttachedPiSession(t *testing.T) {
	dir := t.TempDir()
	secret := "PRIVATE_CHECKPOINT_SENTINEL"
	session := writeTree(t, []string{
		`{"type":"session","version":3,"id":"pi-session"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"go"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"thinking","thinking":"` + secret + `"},{"type":"text","text":"public"}]}}`,
	})
	root := filepath.Join(dir, "agentlab")
	bindAdapterTestManifest(t, root, "checkpoint-exp", "checkpoint-run")
	op, _ := run.Open(root, "checkpoint-exp", "checkpoint-run")
	policy := run.StopPolicy{FirstEventTimeout: time.Second, SoftIdleTimeout: 2 * time.Second, HardIdleTimeout: 3 * time.Second}
	if _, err := Begin(op, session, policy, nil); err != nil {
		t.Fatal(err)
	}
	tree, err := ReadPublicTree(session)
	if err != nil {
		t.Fatal(err)
	}
	userSource, err := tree.Entries[0].EvidenceSource()
	if err != nil {
		t.Fatal(err)
	}
	source, err := tree.Entries[1].EvidenceSource()
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := op.AcquireAdapterWriter(adapterName)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit([]byte("checkpoint-cursor"), run.AdapterBatch{Events: []run.AdapterEvent{{Kind: run.EvidenceUserMessage, SourceLocator: userSource, Label: "user_message", Raw: []byte("go")}, {Kind: run.EvidenceAssistantMessage, SourceLocator: source, Label: "assistant_message", Raw: []byte("public")}}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	evidence, err := op.EvidenceForSourceLocator(source)
	if err != nil {
		t.Fatal(err)
	}
	wrongEvidence, err := op.EvidenceForSourceLocator(userSource)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := artifact.NewStore(filepath.Join(root, "artifacts")).Put([]byte("checkpoint payload"))
	if err != nil {
		t.Fatal(err)
	}
	intent := effect.Intent{ID: "checkpoint-1", RunID: "checkpoint-run", Kind: effect.Checkpoint, Payload: payload}
	if _, err := Checkpoint(op, intent, session, tree.Entries[1].Locator, wrongEvidence, tree.Entries[1].PrefixDigest, testAdapterIdentity()); err == nil {
		t.Fatal("checkpoint accepted evidence for another public entry")
	}
	if _, err := Checkpoint(op, intent, session, tree.Entries[1].Locator, evidence, strings.Repeat("0", 64), testAdapterIdentity()); err == nil {
		t.Fatal("checkpoint accepted a prefix different from the durable selection")
	}
	result, err := Checkpoint(op, intent, session, tree.Entries[1].Locator, evidence, tree.Entries[1].PrefixDigest, testAdapterIdentity())
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := op.RuntimeCheckpointPublicPrefix(result.Checkpoint.Checkpoint)
	if err != nil || prefix != result.Checkpoint.PublicPrefix || prefix.Digest != result.PrefixDigest {
		t.Fatalf("checkpoint public prefix = %#v, %v", prefix, err)
	}
	assertAbsentFromTree(t, root, secret)
}

func testAdapterIdentity() AdapterIdentity {
	digest := strings.Repeat("a", 64)
	return AdapterIdentity{
		Contract: AdapterIdentityContract, PackageName: PinnedPackageName, PackageVersion: PinnedPackageVersion,
		AdapterDigest: digest, BridgeDigest: digest, ContextBuilderDigest: digest, ContextFilterDigest: digest, Provider: "test", Model: "test", ThinkingPolicy: "off", CompactionPolicy: "off",
		Capabilities: []Capability{CapabilityPublicTree, CapabilityArbitraryFork, CapabilityContextSemantics},
	}
}
