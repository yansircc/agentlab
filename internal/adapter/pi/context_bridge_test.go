package pi

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPinnedSDKDoesNotReplayPrivateThinkingToFauxModel(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	artifact := buildContextArtifact(t, source)
	token := "AGENTLAB_PRIVATE_CONTEXT_SENTINEL"
	suffix := "AGENTLAB_SUFFIX_CONTEXT_SENTINEL"
	parent := writeTree(t, []string{
		`{"type":"session","version":3,"id":"parent"}`,
		`{"type":"message","id":"user","parentId":null,"message":{"role":"user","content":"ALPHA"}}`,
		`{"type":"message","id":"assistant","parentId":"user","message":{"role":"assistant","content":[{"type":"thinking","thinking":"` + token + `"},{"type":"text","text":"ready"}]}}`,
		`{"type":"message","id":"poison","parentId":"assistant","message":{"role":"user","content":"` + suffix + `"}}`,
	})
	request, err := json.Marshal(map[string]string{
		"package_root": installedSDKRoot(t), "package_name": PinnedPackageName, "package_version": PinnedPackageVersion,
		"parent_session": parent, "child_session_dir": t.TempDir(), "agent_dir": t.TempDir(), "extension_path": filepath.Join(artifact, "extension.ts"), "entry_id": "assistant", "private_token": token, "suffix_token": suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("node", filepath.Join(filepath.Dir(source), "context_bridge.mjs"))
	command.Stdin = bytes.NewReader(request)
	output, err := command.CombinedOutput()
	if err != nil || !bytes.Equal(output, []byte("{\"contract\":\"agentlab.pi-sdk-context.v1\",\"public_suffix_excluded\":true,\"private_thinking_excluded\":true}\n")) {
		t.Fatalf("Pi SDK context probe = %q, %v", output, err)
	}
}

func buildContextArtifact(t *testing.T, source string) string {
	t.Helper()
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	artifact := t.TempDir()
	if err := os.Mkdir(filepath.Join(artifact, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	extension, err := os.ReadFile(filepath.Join(root, "skills", "agentlab", "extension.ts"))
	if err != nil {
		t.Fatalf("copy context extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "extension.ts"), extension, 0o600); err != nil {
		t.Fatalf("copy context extension: %v", err)
	}
	command := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(artifact, "bin", "agentlab"), "./cmd/agentlab")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build context artifact: %s: %v", output, err)
	}
	return artifact
}
