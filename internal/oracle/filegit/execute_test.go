package filegit

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestFileGitReceiptCapturesContainedFacts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "small.txt"), []byte("content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), bytes.Repeat([]byte{'x'}, 20), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("small.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitPath, _ = filepath.Abs(gitPath)
	init := exec.Command(gitPath, "init", "-q")
	init.Dir = root
	if output, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", output, err)
	}
	store := artifact.NewStore(t.TempDir())
	result, err := Execute(store, Spec{
		Root: root, Paths: []string{"small.txt", "large.txt", "link", "missing.txt"}, MaxFileBytes: 10,
		CaptureGit: true, GitExecutable: gitPath, MaxGitBytes: 4096, SideEffects: []string{"filesystem:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]FileFact{}
	for _, fact := range result.Output.Files {
		byPath[fact.Path] = fact
	}
	if byPath["small.txt"].Content == nil || byPath["large.txt"].Failure == "" || byPath["link"].LinkTarget != "small.txt" || byPath["missing.txt"].Kind != "missing" {
		t.Fatalf("file facts = %#v", result.Output.Files)
	}
	status, _ := store.Read(result.Output.Git.Status)
	if result.Output.Git.ExitCode != 0 || !strings.Contains(string(status), "small.txt") || result.Receipt.Configuration.Digest == "" {
		t.Fatalf("git=%#v status=%q", result.Output.Git, status)
	}
}

func TestFileGitRejectsTraversalAndImplicitBounds(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	root := t.TempDir()
	for _, paths := range [][]string{{"../outside"}, {".."}, {"same", "same"}} {
		if _, err := Execute(store, Spec{Root: root, Paths: paths, MaxFileBytes: 10, SideEffects: []string{"none"}}); err == nil {
			t.Fatalf("invalid paths accepted: %#v", paths)
		}
	}
	if _, err := Execute(store, Spec{Root: root, Paths: []string{"file"}, MaxFileBytes: 0, SideEffects: []string{"none"}}); err == nil {
		t.Fatal("unbounded files were accepted")
	}
}

func TestFileCaptureDoesNotFollowIntermediateSymlinkOutsideRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	store := artifact.NewStore(t.TempDir())
	result, err := Execute(store, Spec{Root: root, Paths: []string{"escape/secret.txt"}, MaxFileBytes: 1024, SideEffects: []string{"filesystem:read"}})
	if err != nil {
		t.Fatal(err)
	}
	fact := result.Output.Files[0]
	if fact.Content != nil || fact.Failure == "" {
		t.Fatalf("outside-root symlink was captured: %#v", fact)
	}
}
