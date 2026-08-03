package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/strictjson"
)

const forkBridgeContract = "agentlab.pi-sdk-fork.v1"

type bridgeForkResponse struct {
	Contract         string `json:"contract"`
	ParentSessionID  string `json:"parent_session_id"`
	ChildSessionID   string `json:"child_session_id"`
	ChildSessionPath string `json:"child_session_path"`
	ChildLeafID      string `json:"child_leaf_id"`
}

func executeForkBridge(attempt forkAttempt) (string, error) {
	bridge, err := os.CreateTemp(attempt.ChildSessionDir, ".agentlab-pi-bridge-*.mjs")
	if err != nil {
		return "", err
	}
	bridgePath := bridge.Name()
	defer os.Remove(bridgePath)
	if err := bridge.Chmod(0o700); err != nil {
		return "", err
	}
	if _, err := bridge.Write(sdkBridge); err != nil {
		return "", err
	}
	if err := bridge.Close(); err != nil {
		return "", err
	}
	request, err := json.Marshal(map[string]string{
		"package_root": attempt.SDKRoot, "package_name": PinnedPackageName, "package_version": PinnedPackageVersion,
		"parent_session": attempt.ParentSession, "child_session_dir": attempt.ChildSessionDir, "entry_id": attempt.EntryID,
	})
	if err != nil {
		return "", err
	}
	command := exec.Command("node", bridgePath)
	command.Stdin = bytes.NewReader(request)
	output, err := command.Output()
	if err != nil {
		return "", errors.New("Pi SDK fork failed")
	}
	var response bridgeForkResponse
	if strictjson.Decode(output, &response) != nil || response.Contract != forkBridgeContract || response.ParentSessionID != attempt.ParentSessionID || response.ChildSessionID == "" || response.ChildLeafID != attempt.EntryID || !withinDirectory(attempt.ChildSessionDir, response.ChildSessionPath) {
		return "", errors.New("Pi SDK fork receipt is invalid")
	}
	return response.ChildSessionPath, nil
}

func withinDirectory(directory, path string) bool {
	base, err := filepath.Abs(directory)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(base, target)
	return err == nil && relative != "." && relative != ".." && !bytes.HasPrefix([]byte(relative), []byte(".."+string(filepath.Separator)))
}
