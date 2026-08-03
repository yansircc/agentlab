package oracle

import (
	"testing"

	"github.com/yansircc/agentlab/internal/artifact"
)

func TestReceiptBindsCanonicalFactsAndExplicitEffects(t *testing.T) {
	store := artifact.NewStore(t.TempDir())
	receipt, err := Record(store, "test", map[string]any{"b": 2, "a": 1}, map[string]bool{"ok": true}, []string{"write:b", "read:a"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Contract != ReceiptContract || receipt.Engine.Digest == "" || receipt.Configuration.Digest == "" || receipt.Output.Digest == "" || receipt.SideEffects.Digest == "" {
		t.Fatalf("receipt = %#v", receipt)
	}
	if _, err := Record(store, "test", struct{}{}, struct{}{}, nil); err == nil {
		t.Fatal("implicit side effects were accepted")
	}
	if _, err := Record(store, "test", struct{}{}, struct{}{}, []string{"same", "same"}); err == nil {
		t.Fatal("duplicate side effects were accepted")
	}
}
