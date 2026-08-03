package preparation

import (
	"testing"
	"time"
)

func TestReplayRejectsUnknownEventDataAndPostSealMutation(t *testing.T) {
	op := begunOperation(t, "corrupt")
	if _, err := op.ledger.Append(time.Now().UTC(), eventNode, map[string]any{
		"id": "unknown-field", "fact": map[string]string{"query": "q"}, "shadow": true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Status(); err == nil {
		t.Fatal("unknown event data field was accepted")
	}

	op = begunOperation(t, "post-seal")
	recordCleanAssay(t, op)
	basis, _ := op.ChallengeBasis()
	if err := op.Challenge(Challenge{Basis: basis}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Seal(); err != nil {
		t.Fatal(err)
	}
	if _, err := op.ledger.Append(time.Now().UTC(), eventFact, RepositoryFact{ID: "late", Statement: "late"}); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Status(); err == nil {
		t.Fatal("event after seal was accepted")
	}
}
