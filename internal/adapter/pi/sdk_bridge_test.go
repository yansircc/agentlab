package pi

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type sdkForkResponse struct {
	Contract         string `json:"contract"`
	ParentSessionID  string `json:"parent_session_id"`
	ChildSessionID   string `json:"child_session_id"`
	ChildSessionPath string `json:"child_session_path"`
	ChildLeafID      string `json:"child_leaf_id"`
}

func TestPinnedSDKForksExactAssistantPublicPrefix(t *testing.T) {
	sdkRoot := installedSDKRoot(t)
	parent := writeTree(t, []string{
		`{"type":"session","version":3,"id":"parent"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"ALPHA"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"text","text":"ready"}]}}`,
		`{"type":"message","id":"poison","parentId":"assistant","message":{"role":"user","content":"POISON"}}`,
	})
	childDir := t.TempDir()
	request, err := json.Marshal(map[string]string{
		"package_root": sdkRoot, "package_name": "@earendil-works/pi-coding-agent", "package_version": "0.83.0",
		"parent_session": parent, "child_session_dir": childDir, "entry_id": "assistant",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, source, _, _ := runtime.Caller(0)
	bridge := filepath.Join(filepath.Dir(source), "sdk_bridge.mjs")
	command := exec.Command("node", bridge)
	command.Stdin = bytes.NewReader(request)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var response sdkForkResponse
	if err := json.Unmarshal(output, &response); err != nil || response.Contract != "agentlab.pi-sdk-fork.v1" || response.ParentSessionID != "parent" || response.ChildSessionID == "" || response.ChildLeafID != "assistant" {
		t.Fatalf("SDK fork response = %#v, %v", response, err)
	}
	parentTree, err := ReadPublicTree(parent)
	if err != nil {
		t.Fatal(err)
	}
	childTree, err := ReadPublicTree(response.ChildSessionPath)
	if err != nil {
		t.Fatal(err)
	}
	parentPrefix, _, _, err := parentTree.Checkpoint(parentTree.Entries[1].Locator)
	if err != nil {
		t.Fatal(err)
	}
	childPrefix, _, _, err := childTree.Checkpoint(childTree.Entries[len(childTree.Entries)-1].Locator)
	if err != nil || !bytes.Equal(parentPrefix, childPrefix) || len(parentTree.Entries) != 3 || len(childTree.Entries) != 2 {
		t.Fatalf("SDK fork public prefix = %q, parent=%d child=%d, %v", childPrefix, len(parentTree.Entries), len(childTree.Entries), err)
	}
}

func TestDiscoverIdentityBindsInstalledSDKAndBridge(t *testing.T) {
	identity, err := DiscoverIdentity(IdentityConfig{
		SDKRoot: installedSDKRoot(t), AdapterDigest: strings.Repeat("a", 64), Provider: "test", Model: "test", ThinkingPolicy: "off", CompactionPolicy: "off",
	})
	if err != nil || identity.PackageVersion != PinnedPackageVersion || identity.BridgeDigest != sha256Digest(sdkBridge) || !digest(identity.ContextBuilderDigest) {
		t.Fatalf("Pi adapter identity = %#v, %v", identity, err)
	}
}

func installedSDKRoot(t *testing.T) string {
	t.Helper()
	pi, err := exec.LookPath("pi")
	if err != nil {
		t.Skip("Pi is not installed")
	}
	target, err := filepath.EvalSymlinks(pi)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(target))
	if _, err := exec.Command("node", "-e", `const p=process.argv[1];const x=require(p);if(x.name!=="@earendil-works/pi-coding-agent"||x.version!=="0.83.0")process.exit(1)`, filepath.Join(root, "package.json")).Output(); err != nil {
		t.Skip("the installed Pi SDK is not the pinned 0.83.0 package")
	}
	return root
}
