package pi

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicTreeExcludesThinkingAndClosesToolCausality(t *testing.T) {
	secret := "PRIVATE_TREE_SENTINEL"
	path := writeTree(t, []string{
		`{"type":"session","version":3,"id":"session-1"}`,
		`{"type":"message","id":"user-1","parentId":null,"message":{"role":"user","content":"inspect"}}`,
		`{"type":"message","id":"assistant-1","parentId":"user-1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"` + secret + `"},{"type":"text","text":"public"},{"type":"toolCall","id":"call-1","name":"status","arguments":{}}]}}`,
		`{"type":"message","id":"result-1","parentId":"assistant-1","message":{"role":"toolResult","toolCallId":"call-1","toolName":"status","content":[{"type":"text","text":"ok"}]}}`,
		`{"type":"message","id":"assistant-2","parentId":"result-1","message":{"role":"assistant","content":[{"type":"text","text":"continue"}]}}`,
	})
	tree, err := ReadPublicTree(path)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(tree)
	if err != nil || strings.Contains(string(encoded), secret) {
		t.Fatalf("public tree leaked thinking: %s", encoded)
	}
	if len(tree.Entries) != 4 || tree.Entries[1].StructurallyForkable || !tree.Entries[2].StructurallyForkable || !tree.Entries[3].StructurallyForkable {
		t.Fatalf("forkable entries = %#v", tree.Entries)
	}
	prefix, state, digest, err := tree.Checkpoint(tree.Entries[2].Locator)
	if err != nil || digest != tree.Entries[2].PrefixDigest || sha256Digest(prefix) != digest || strings.Contains(string(prefix), secret) || strings.Contains(string(state), secret) {
		t.Fatalf("checkpoint = %q, %q, %q, %v", prefix, state, digest, err)
	}
}

func TestPublicTreeRejectsOpenOrUnknownContext(t *testing.T) {
	open := writeTree(t, []string{
		`{"type":"session","version":3,"id":"session-1"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"go"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"toolCall","id":"call","name":"x","arguments":{}}]}}`,
	})
	tree, err := ReadPublicTree(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := tree.Checkpoint(tree.Entries[1].Locator); !errors.Is(err, ErrEntryNotForkable) {
		t.Fatalf("open tool checkpoint error = %v", err)
	}
	unknown := writeTree(t, []string{
		`{"type":"session","version":3,"id":"session-2"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"go"}}`,
		`{"type":"custom_message","id":"custom","parentId":"user"}`,
		`{"type":"message","id":"assistant","parentId":"custom","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`,
	})
	tree, err = ReadPublicTree(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := tree.Checkpoint(tree.Entries[1].Locator); !errors.Is(err, ErrEntryNotForkable) {
		t.Fatalf("unknown-context checkpoint error = %v", err)
	}
}

func TestPublicTreeRejectsBrokenToolOrParentLinks(t *testing.T) {
	cases := [][]string{
		{
			`{"type":"session","version":3,"id":"session-1"}`,
			`{"type":"message","id":"result","parentId":null,"message":{"role":"toolResult","toolCallId":"missing","toolName":"x","content":"no"}}`,
		},
		{
			`{"type":"session","version":3,"id":"session-2"}`,
			`{"type":"message","id":"child","parentId":"missing","message":{"role":"user","content":"no"}}`,
		},
	}
	for _, records := range cases {
		if _, err := ReadPublicTree(writeTree(t, records)); err == nil {
			t.Fatalf("invalid tree was accepted: %v", records)
		}
	}
}

func TestAdapterIdentityRequiresExactSDKAndCapabilities(t *testing.T) {
	identity := testAdapterIdentity()
	if err := identity.Validate(); err != nil {
		t.Fatal(err)
	}
	identity.PackageVersion = "0.84.0"
	if err := identity.Validate(); err == nil {
		t.Fatal("unverified Pi version was accepted")
	}
	identity = testAdapterIdentity()
	identity.Capabilities = []Capability{CapabilityPublicTree, CapabilityContextSemantics}
	if err := identity.Validate(); err == nil {
		t.Fatal("user-boundary substitute capability was accepted")
	}
}

func writeTree(t *testing.T, records []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(records, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
