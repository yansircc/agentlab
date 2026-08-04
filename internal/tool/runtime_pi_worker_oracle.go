package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/run"
	"github.com/yansircc/agentlab/internal/strictjson"
)

// HostOracleKind is a Host-private, closed selection of the objective oracle
// producer attached to a Worker runtime profile. It is never projected into a
// provider tool schema or accepted from model-authored input.
type HostOracleKind string

const (
	HostOracleNone      HostOracleKind = ""
	HostOracleDeployctl HostOracleKind = "deployctl"
)

func (value HostOracleKind) Valid() bool {
	return value == HostOracleNone || value == HostOracleDeployctl
}

type hostWorkerOracle func(HostOracleKind, string, string) error

type hostWorkerOracleCommand func(binary, directory string, args []string) ([]byte, error)

type hostWorkerOracleOutput struct {
	OK   bool `json:"ok"`
	Data struct {
		RunID    string          `json:"run_id"`
		Evidence run.EvidenceRef `json:"evidence"`
	} `json:"data"`
}

// newHostWorkerOracle is installed only when a Host-private runtime plan is
// opened from disk. It calls the exact running bundled binary, verifies its
// preflight-bound digest, and accepts only the Host-only oracle projection.
func newHostWorkerOracle(planPath string) hostWorkerOracle {
	return newHostWorkerOracleWithCommand(planPath, os.Executable, runHostWorkerOracleCommand)
}

func newHostWorkerOracleWithCommand(planPath string, executable func() (string, error), invoke hostWorkerOracleCommand) hostWorkerOracle {
	hostRoot := filepath.Dir(planPath)
	return func(kind HostOracleKind, runID, expectedBinaryDigest string) error {
		if kind != HostOracleDeployctl || runID == "" || !validHostOracleDigest(expectedBinaryDigest) || executable == nil || invoke == nil {
			return errors.New("Host Worker oracle is invalid")
		}
		binary, err := executable()
		if err != nil || executableDigest(binary) != expectedBinaryDigest {
			return errors.New("Host Worker oracle identity differs from runtime binding")
		}
		output, err := invoke(binary, hostRoot, []string{"acceptance", "worker-oracle", "-host-root", hostRoot, "-run-id", runID})
		if err != nil {
			return errors.New("Host Worker oracle invocation failed")
		}
		var result hostWorkerOracleOutput
		if strictjson.Decode(output, &result) != nil || !result.OK || result.Data.RunID != runID || result.Data.Evidence.ExperimentID == "" || result.Data.Evidence.RunID != runID || result.Data.Evidence.Sequence == 0 || result.Data.Evidence.Item < 0 {
			return errors.New("Host Worker oracle result is invalid")
		}
		return nil
	}
}

func runHostWorkerOracleCommand(binary, directory string, args []string) ([]byte, error) {
	command := exec.Command(binary, args...)
	command.Dir = directory
	command.Env = []string{}
	return command.Output()
}

func executableDigest(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validHostOracleDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, item := range value {
		if !(item >= 'a' && item <= 'f' || item >= '0' && item <= '9') {
			return false
		}
	}
	return true
}
