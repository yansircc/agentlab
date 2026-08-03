package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOnceIsIdempotentOnlyForExactBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	if err := WriteOnce(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOnce(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("exact replay failed: %v", err)
	}
	if err := WriteOnce(path, []byte("second"), 0o600); !errors.Is(err, ErrValueExists) {
		t.Fatalf("changed replay error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first" {
		t.Fatalf("stored bytes = %q, err = %v", data, err)
	}
}
