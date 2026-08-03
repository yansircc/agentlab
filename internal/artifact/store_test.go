package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreIdentityAndVerification(t *testing.T) {
	store := NewStore(t.TempDir())
	first, err := store.Put([]byte("same bytes"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put([]byte("same bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("same bytes produced different refs: %#v %#v", first, second)
	}
	if err := os.WriteFile(filepath.Join(store.root, "sha256", first.Digest), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(first); err == nil {
		t.Fatal("tampered artifact was accepted")
	}
}

func TestStoreRejectsRefIssuedByAnotherCapabilityRoot(t *testing.T) {
	evaluated := NewStore(t.TempDir())
	audit := NewStore(t.TempDir())
	ref, err := audit.Put([]byte("private ground truth"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evaluated.Read(ref); err == nil {
		t.Fatal("cross-root artifact ref was accepted")
	}
	if evaluated.Scope() == audit.Scope() {
		t.Fatal("distinct roots share an identity")
	}
}
