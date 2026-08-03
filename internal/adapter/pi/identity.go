package pi

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

//go:embed sdk_bridge.mjs
var sdkBridge []byte

type IdentityConfig struct {
	SDKRoot           string `json:"sdk_root"`
	AdapterDigest     string `json:"adapter_digest"`
	ContextFilterPath string `json:"context_filter_path"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	ThinkingPolicy    string `json:"thinking_policy"`
	CompactionPolicy  string `json:"compaction_policy"`
}

func DiscoverIdentity(config IdentityConfig) (AdapterIdentity, error) {
	if !filepath.IsAbs(config.SDKRoot) || !filepath.IsAbs(config.ContextFilterPath) || !digest(config.AdapterDigest) || !identityText(config.Provider) || !identityText(config.Model) || !identityText(config.ThinkingPolicy) || !identityText(config.CompactionPolicy) {
		return AdapterIdentity{}, errors.New("Pi identity configuration is invalid")
	}
	data, err := os.ReadFile(filepath.Join(config.SDKRoot, "package.json"))
	if err != nil {
		return AdapterIdentity{}, err
	}
	var packageInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &packageInfo) != nil || packageInfo.Name != PinnedPackageName || packageInfo.Version != PinnedPackageVersion {
		return AdapterIdentity{}, errors.New("Pi SDK package is not pinned")
	}
	contextBuilder, err := os.ReadFile(filepath.Join(config.SDKRoot, "dist", "core", "session-manager.js"))
	if err != nil {
		return AdapterIdentity{}, err
	}
	contextFilter, err := os.ReadFile(config.ContextFilterPath)
	if err != nil {
		return AdapterIdentity{}, err
	}
	identity := AdapterIdentity{
		Contract: AdapterIdentityContract, PackageName: packageInfo.Name, PackageVersion: packageInfo.Version,
		AdapterDigest: config.AdapterDigest, BridgeDigest: sha256Digest(sdkBridge), ContextBuilderDigest: sha256Digest(contextBuilder), ContextFilterDigest: sha256Digest(contextFilter),
		Provider: config.Provider, Model: config.Model, ThinkingPolicy: config.ThinkingPolicy, CompactionPolicy: config.CompactionPolicy,
		Capabilities: []Capability{CapabilityPublicTree, CapabilityArbitraryFork, CapabilityContextSemantics},
	}
	return identity, identity.Validate()
}
