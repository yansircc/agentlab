package processidentity

import (
	"os"
	"syscall"
	"testing"
)

func TestSystemProberRequiresFullIdentity(t *testing.T) {
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	identity, err := Capture(os.Getpid(), pgid, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := (SystemProber{}).Observe(identity); got != Matches {
		t.Fatalf("live identity = %s", got)
	}
	identity.StartToken += " reused"
	if got := (SystemProber{}).Observe(identity); got != Mismatch {
		t.Fatalf("reused PID identity = %s", got)
	}
}
