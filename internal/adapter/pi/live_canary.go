package pi

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/yansircc/agentlab/internal/strictjson"
)

const LiveCanaryContract = "agentlab.pi-live-context-canary.v1"

//go:embed live_canary.mjs
var liveCanaryBridge []byte

// LiveCanarySpec is Host-only. Its paths and provider configuration never
// cross a Supervisor tool boundary or enter an evaluated receipt.
type LiveCanarySpec struct {
	NodePath      string
	SDKRoot       string
	ExtensionPath string
	BinaryPath    string
	Identity      AdapterIdentity
}

// LiveCanaryReceipt deliberately contains no model text, session locator, or
// sentinel. Persistence binds these two booleans to an artifact identity.
type LiveCanaryReceipt struct {
	Contract                string `json:"contract"`
	PublicSuffixExcluded    bool   `json:"public_suffix_excluded"`
	PrivateThinkingExcluded bool   `json:"private_thinking_excluded"`
}

func (value LiveCanaryReceipt) Validate() error {
	if value.Contract != LiveCanaryContract || !value.PublicSuffixExcluded || !value.PrivateThinkingExcluded {
		return errors.New("Pi live context canary failed")
	}
	return nil
}

// RunLiveCanary proves the exact bundled extension and final selected model
// see the selected public prefix but not its private-thinking block or suffix.
// Failure returns no reusable receipt and every temporary session is removed.
func RunLiveCanary(spec LiveCanarySpec) (LiveCanaryReceipt, error) {
	if err := spec.validate(); err != nil {
		return LiveCanaryReceipt{}, err
	}
	discovered, err := VerifyRuntimeIdentity(IdentityConfig{SDKRoot: spec.SDKRoot, ContextFilterPath: spec.ExtensionPath, AdapterDigest: spec.Identity.AdapterDigest, Provider: spec.Identity.Provider, Model: spec.Identity.Model, ThinkingPolicy: spec.Identity.ThinkingPolicy, CompactionPolicy: spec.Identity.CompactionPolicy})
	if err != nil || !reflect.DeepEqual(discovered, spec.Identity) {
		return LiveCanaryReceipt{}, errors.New("Pi live context canary identity differs from Host binding")
	}
	return runLiveCanaryBridge(spec, liveCanaryBridge)
}

func (spec LiveCanarySpec) validate() error {
	if spec.Identity.Validate() != nil || spec.Identity.CompactionPolicy != "off" {
		return errors.New("Pi live context canary configuration is invalid")
	}
	for _, path := range []string{spec.NodePath, spec.SDKRoot, spec.ExtensionPath, spec.BinaryPath} {
		if !filepath.IsAbs(path) {
			return errors.New("Pi live context canary path is invalid")
		}
	}
	if filepath.Clean(spec.BinaryPath) != filepath.Join(filepath.Dir(spec.ExtensionPath), "bin", "agentlab") {
		return errors.New("Pi live context canary artifact is invalid")
	}
	return nil
}

func runLiveCanaryBridge(spec LiveCanarySpec, bridge []byte) (LiveCanaryReceipt, error) {
	root, err := os.MkdirTemp("", "agentlab-pi-live-canary-")
	if err != nil {
		return LiveCanaryReceipt{}, err
	}
	defer os.RemoveAll(root)
	bridgePath := filepath.Join(root, "live-canary.mjs")
	if err := os.WriteFile(bridgePath, bridge, 0o600); err != nil {
		return LiveCanaryReceipt{}, err
	}
	request, err := json.Marshal(struct {
		PackageRoot    string `json:"package_root"`
		PackageName    string `json:"package_name"`
		PackageVersion string `json:"package_version"`
		ExtensionPath  string `json:"extension_path"`
		NodePath       string `json:"node_path"`
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		Thinking       string `json:"thinking"`
		Compaction     string `json:"compaction"`
		WorkRoot       string `json:"work_root"`
	}{spec.SDKRoot, spec.Identity.PackageName, spec.Identity.PackageVersion, spec.ExtensionPath, spec.NodePath, spec.Identity.Provider, spec.Identity.Model, spec.Identity.ThinkingPolicy, spec.Identity.CompactionPolicy, filepath.Join(root, "session")})
	if err != nil {
		return LiveCanaryReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, spec.NodePath, bridgePath)
	command.Stdin = bytes.NewReader(request)
	output, err := command.Output()
	if err != nil || ctx.Err() != nil {
		return LiveCanaryReceipt{}, errors.New("Pi live context canary execution failed")
	}
	var receipt LiveCanaryReceipt
	if strictjson.Decode(output, &receipt) != nil || receipt.Validate() != nil {
		return LiveCanaryReceipt{}, errors.New("Pi live context canary receipt is invalid")
	}
	return receipt, nil
}
