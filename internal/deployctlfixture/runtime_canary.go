package deployctlfixture

import (
	"errors"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const runtimeCanaryContract = "agentlab.deployctl-live-canary.v1"

type liveCanaryRunner func(piadapter.LiveCanarySpec) (piadapter.LiveCanaryReceipt, error)

// runtimeCanaryReceipt binds the boolean semantic proof to the sole immutable
// AdapterIdentity artifact. It intentionally has no token, session, or output.
type runtimeCanaryReceipt struct {
	Contract                string       `json:"contract"`
	Adapter                 artifact.Ref `json:"adapter"`
	PublicSuffixExcluded    bool         `json:"public_suffix_excluded"`
	PrivateThinkingExcluded bool         `json:"private_thinking_excluded"`
}

func bindLiveCanary(store artifact.Store, spec piadapter.LiveCanarySpec, adapter artifact.Ref, runner liveCanaryRunner) (artifact.Ref, error) {
	if !adapter.Valid() || runner == nil {
		return artifact.Ref{}, errors.New("deployctl live canary is invalid")
	}
	receipt, err := runner(spec)
	if err != nil || receipt.Validate() != nil {
		return artifact.Ref{}, errors.New("deployctl live canary failed")
	}
	return putCanonical(store, runtimeCanaryReceipt{Contract: runtimeCanaryContract, Adapter: adapter, PublicSuffixExcluded: receipt.PublicSuffixExcluded, PrivateThinkingExcluded: receipt.PrivateThinkingExcluded})
}

func verifyLiveCanary(store artifact.Store, ref artifact.Ref) (piadapter.AdapterIdentity, error) {
	data, err := store.Read(ref)
	if err != nil {
		return piadapter.AdapterIdentity{}, err
	}
	var receipt runtimeCanaryReceipt
	if strictjson.Decode(data, &receipt) != nil || receipt.Contract != runtimeCanaryContract || !receipt.Adapter.Valid() || !receipt.PublicSuffixExcluded || !receipt.PrivateThinkingExcluded {
		return piadapter.AdapterIdentity{}, errors.New("deployctl live canary receipt is invalid")
	}
	identityData, err := store.Read(receipt.Adapter)
	if err != nil {
		return piadapter.AdapterIdentity{}, err
	}
	var identity piadapter.AdapterIdentity
	if strictjson.Decode(identityData, &identity) != nil || identity.Validate() != nil {
		return piadapter.AdapterIdentity{}, errors.New("deployctl live canary identity is invalid")
	}
	return identity, nil
}
