package pi

import (
	"encoding/hex"
	"encoding/json"
	"errors"
)

const maxTreeEntries = 10000

var ErrEntryNotForkable = errors.New("Pi public entry is not structurally forkable")

const (
	AdapterIdentityContract = "agentlab.pi-adapter-identity.v1"
	PinnedPackageName       = "@earendil-works/pi-coding-agent"
	PinnedPackageVersion    = "0.83.0"
)

type Capability string

const (
	CapabilityPublicTree       Capability = "public-tree"
	CapabilityArbitraryFork    Capability = "fork-arbitrary-public-entry"
	CapabilityContextSemantics Capability = "context-semantics-canary"
)

type AdapterIdentity struct {
	Contract             string       `json:"contract"`
	PackageName          string       `json:"package_name"`
	PackageVersion       string       `json:"package_version"`
	AdapterDigest        string       `json:"adapter_digest"`
	BridgeDigest         string       `json:"bridge_digest"`
	ContextBuilderDigest string       `json:"context_builder_digest"`
	ContextFilterDigest  string       `json:"context_filter_digest"`
	Provider             string       `json:"provider"`
	Model                string       `json:"model"`
	ThinkingPolicy       string       `json:"thinking_policy"`
	CompactionPolicy     string       `json:"compaction_policy"`
	Capabilities         []Capability `json:"capabilities"`
}

func (i AdapterIdentity) Validate() error {
	if i.Contract != AdapterIdentityContract || i.PackageName != PinnedPackageName || i.PackageVersion != PinnedPackageVersion || !digest(i.AdapterDigest) || !digest(i.BridgeDigest) || !digest(i.ContextBuilderDigest) || !digest(i.ContextFilterDigest) || !identityText(i.Provider) || !identityText(i.Model) || !identityText(i.ThinkingPolicy) || !identityText(i.CompactionPolicy) {
		return errors.New("Pi adapter identity is invalid")
	}
	required := map[Capability]bool{CapabilityPublicTree: false, CapabilityArbitraryFork: false, CapabilityContextSemantics: false}
	for _, capability := range i.Capabilities {
		present, known := required[capability]
		if !known || present {
			return errors.New("Pi adapter capability is invalid")
		}
		required[capability] = true
	}
	for _, present := range required {
		if !present {
			return errors.New("Pi adapter capability is incomplete")
		}
	}
	return nil
}

func digest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func identityText(value string) bool { return value != "" && len(value) <= 256 }

type PublicTree struct {
	RuntimeLocator string        `json:"runtime_locator"`
	Entries        []PublicEntry `json:"entries"`
	sessionID      string
	nodes          map[string]treeNode
}

type PublicEntry struct {
	Locator              string `json:"locator"`
	RuntimeLocator       string `json:"runtime_locator"`
	ParentLocator        string `json:"parent_locator,omitempty"`
	Role                 string `json:"role"`
	Kind                 string `json:"kind"`
	PublicText           string `json:"public_text"`
	PrefixDigest         string `json:"prefix_digest"`
	StructurallyForkable bool   `json:"structurally_forkable"`
}

type treeNode struct {
	id       string
	parentID string
	entry    *PublicEntry
	prefix   []prefixEntry
	pending  map[string]bool
	valid    bool
}

type prefixEntry struct {
	Role       string `json:"role"`
	Kind       string `json:"kind"`
	PublicText string `json:"public_text"`
}

type treeRecord struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	ParentID *string         `json:"parentId"`
	Message  json.RawMessage `json:"message"`
}

type publicPayload struct {
	Role string `json:"role"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type checkpointState struct {
	RuntimeLocator string `json:"runtime_locator"`
	SessionID      string `json:"session_id"`
	EntryID        string `json:"entry_id"`
	PrefixDigest   string `json:"prefix_digest"`
}

func (t PublicTree) SessionID() string { return t.sessionID }
