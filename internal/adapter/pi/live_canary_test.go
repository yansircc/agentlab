package pi

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLiveCanaryBridgeReturnsOnlyBooleanReceipt(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	t.Setenv("AGENTLAB_CANARY_CREDENTIAL", "test-credential")
	bridge := []byte(`import{readFileSync}from"node:fs";const v=JSON.parse(readFileSync(0,"utf8"));const keys=["package_root","package_name","package_version","extension_path","node_path","provider","model","thinking","compaction","work_root"];if(Object.keys(v).length!==keys.length||keys.some(k=>typeof v[k]!=="string")||process.env.PROVIDER_TOKEN!=="test-credential"||Object.keys(process.env).some(k=>!["PROVIDER_TOKEN","HOME","PI_CODING_AGENT_DIR","PI_CODING_AGENT_SESSION_DIR"].includes(k)))process.exit(1);process.stdout.write('{"contract":"agentlab.pi-live-context-canary.v1","public_suffix_excluded":true,"private_thinking_excluded":true}\n')`)
	value, err := runLiveCanaryBridge(LiveCanarySpec{NodePath: node, SDKRoot: "/sdk", ExtensionPath: "/skill/extension.ts", BinaryPath: "/skill/bin/agentlab", ProviderCredentialEnv: "PROVIDER_TOKEN", CredentialHandle: "AGENTLAB_CANARY_CREDENTIAL", Identity: liveCanaryIdentity()}, bridge)
	if err != nil || value.Validate() != nil {
		t.Fatalf("live canary bridge = %#v, %v", value, err)
	}
}

func TestLiveCanaryRejectsUnbundledArtifactAndNonOffCompaction(t *testing.T) {
	value := LiveCanarySpec{NodePath: "/node", SDKRoot: "/sdk", ExtensionPath: "/skill/extension.ts", BinaryPath: "/other/agentlab", ProviderCredentialEnv: "PROVIDER_TOKEN", CredentialHandle: "AGENTLAB_CANARY_CREDENTIAL", Identity: liveCanaryIdentity()}
	if value.validate() == nil {
		t.Fatal("live canary accepted an unbundled binary")
	}
	value.BinaryPath = "/skill/bin/agentlab"
	value.Identity.CompactionPolicy = "auto"
	if value.validate() == nil {
		t.Fatal("live canary accepted an uncontrolled compaction policy")
	}
}

func TestLiveCanaryBridgeRejectsMissingCredentialHandle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is not installed")
	}
	t.Setenv("AMBIENT_PROVIDER_TOKEN", "must-not-be-used")
	value, err := runLiveCanaryBridge(LiveCanarySpec{NodePath: node, SDKRoot: "/sdk", ExtensionPath: "/skill/extension.ts", BinaryPath: "/skill/bin/agentlab", ProviderCredentialEnv: "PROVIDER_TOKEN", CredentialHandle: "MISSING_PROVIDER_CREDENTIAL", Identity: liveCanaryIdentity()}, []byte(`process.stdout.write('{}')`))
	if err == nil || value != (LiveCanaryReceipt{}) {
		t.Fatal("live canary used an ambient provider credential")
	}
}

func TestVerifyRuntimeIdentityBindsAdjacentBundledBinary(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	artifact := buildContextArtifact(t, source)
	binary := filepath.Join(artifact, "bin", "agentlab")
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	config := IdentityConfig{SDKRoot: installedSDKRoot(t), ContextFilterPath: filepath.Join(artifact, "extension.ts"), AdapterDigest: sha256Digest(data), Provider: "test", Model: "test", ThinkingPolicy: "off", CompactionPolicy: "off"}
	identity, err := VerifyRuntimeIdentity(config)
	if err != nil || identity.AdapterDigest != config.AdapterDigest {
		t.Fatalf("runtime identity = %#v, %v", identity, err)
	}
	if err := os.WriteFile(binary, append(data, '\n'), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRuntimeIdentity(config); err == nil {
		t.Fatal("runtime identity accepted a changed bundled binary")
	}
}

func TestLiveCanaryBridgeUsesFinalProviderWithoutFallback(t *testing.T) {
	data, err := os.ReadFile("live_canary.mjs")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{"--provider", "--model", "--thinking", "--no-tools", "--extension", "createBranchedSession", "AGENTLAB_CONTEXT_FILTER_ONLY"} {
		if !strings.Contains(source, required) {
			t.Fatalf("live canary omitted %q", required)
		}
	}
	if strings.Contains(source, "fauxProvider") || strings.Contains(source, "private_token") || strings.Contains(source, "suffix_token") {
		t.Fatal("live canary contains a synthetic-provider or caller-supplied sentinel path")
	}
}

func liveCanaryIdentity() AdapterIdentity {
	digest := strings.Repeat("a", 64)
	return AdapterIdentity{Contract: AdapterIdentityContract, PackageName: PinnedPackageName, PackageVersion: PinnedPackageVersion, AdapterDigest: digest, BridgeDigest: digest, ContextBuilderDigest: digest, ContextFilterDigest: digest, Provider: "provider", Model: "model", ThinkingPolicy: "off", CompactionPolicy: "off", Capabilities: []Capability{CapabilityPublicTree, CapabilityArbitraryFork, CapabilityContextSemantics}}
}
