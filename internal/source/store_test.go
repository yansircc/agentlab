package source

import (
	"encoding/json"
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestSnapshotIdentityIsPathSortedAndContentAddressed(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	first, err := Build(store, []InputFile{{Path: "b.go", Content: []byte("b")}, {Path: "a.go", Content: []byte("a")}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(store, []InputFile{{Path: "a.go", Content: []byte("a")}, {Path: "b.go", Content: []byte("b")}})
	if err != nil || second != first {
		t.Fatalf("snapshot identity drifted with input order: %#v %#v, %v", first, second, err)
	}
	snapshot, err := Load(store, first)
	if err != nil || len(snapshot.Files) != 2 || snapshot.Files[0].Path != "a.go" || !snapshot.Contains("b.go", snapshot.Files[1].Artifact) {
		t.Fatalf("snapshot = %#v, %v", snapshot, err)
	}
}

func TestSnapshotRejectsDuplicateEscapeAndAbsentMembers(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	if _, err := Build(store, []InputFile{{Path: "same.go"}, {Path: "same.go"}}); err == nil {
		t.Fatal("duplicate source path accepted")
	}
	if _, err := Build(store, []InputFile{{Path: "../outside.go"}}); err == nil {
		t.Fatal("escaping source path accepted")
	}
	absent := artifact.Ref{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
	data, _ := json.Marshal(Snapshot{Contract: Contract, Files: []File{{Path: "owner.go", Artifact: absent}}})
	ref, _ := store.Put(data)
	if _, err := Load(store, ref); err == nil {
		t.Fatal("snapshot with absent member artifact accepted")
	}
}
