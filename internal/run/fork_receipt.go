package run

import (
	"errors"

	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/strictjson"
)

// ForkReceipt returns the one settled generic fork receipt and its
// adapter-private child-session payload. It is for Host adapter recovery,
// never a provider-tool projection.
func (o *Operation) ForkReceipt(intentID string) (SessionForked, []byte, effect.Receipt, error) {
	state, err := o.currentState()
	if err != nil {
		return SessionForked{}, nil, effect.Receipt{}, err
	}
	receipt, exists := state.effectReceipts[intentID]
	if !exists || receipt.IntentID != intentID || receipt.Kind != effect.Fork {
		return SessionForked{}, nil, effect.Receipt{}, errors.New("fork effect receipt is unavailable")
	}
	evidence, err := o.artifacts.Read(receipt.Evidence)
	if err != nil {
		return SessionForked{}, nil, effect.Receipt{}, err
	}
	var forked SessionForked
	if strictjson.Decode(evidence, &forked) != nil || forked.Validate() != nil || forked.Intent.ID != intentID || forked.Intent.RunID != o.runID {
		return SessionForked{}, nil, effect.Receipt{}, errors.New("fork effect receipt is invalid")
	}
	if recorded, exists := state.sessionForks[forked.ChildSession]; !exists || recorded != forked {
		return SessionForked{}, nil, effect.Receipt{}, errors.New("fork receipt differs from run ledger")
	}
	child, err := o.artifacts.Read(forked.ChildSession)
	if err != nil {
		return SessionForked{}, nil, effect.Receipt{}, err
	}
	return forked, child, receipt, nil
}

// VerifyForkEffect proves that the settled fork intent owns the durable child
// session receipt and that its receipt is the exact observed fork outcome.
func (o *Operation) VerifyForkEffect(intent effect.Intent) (SessionForked, error) {
	if intent.RunID != o.runID || intent.Kind != effect.Fork || intent.Validate() != nil {
		return SessionForked{}, errors.New("fork effect intent is invalid")
	}
	if _, _, err := o.verifyObservedReceipt(intent); err != nil {
		return SessionForked{}, err
	}
	forked, _, receipt, err := o.ForkReceipt(intent.ID)
	if err != nil || receipt.Kind != intent.Kind || forked.Intent != intent {
		return SessionForked{}, errors.New("fork effect differs from receipt")
	}
	return forked, nil
}
