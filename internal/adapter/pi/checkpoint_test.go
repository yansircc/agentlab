package pi

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	result, err := Checkpoint(op, session, tree.Entries[1].Locator, testAdapterIdentity())
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
		AdapterDigest: digest, BridgeDigest: digest, ContextBuilderDigest: digest, Provider: "test", Model: "test", ThinkingPolicy: "off", CompactionPolicy: "off",
		Capabilities: []Capability{CapabilityPublicTree, CapabilityArbitraryFork, CapabilityContextSemantics},
	}
}
