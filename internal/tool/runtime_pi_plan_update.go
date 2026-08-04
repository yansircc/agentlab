package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/transaction"
)

// LoadPiRuntimeHost opens a Host-private plan. Provider requests never carry
// its path; the CLI receives it only from the extension's Host binding.
func LoadPiRuntimeHost(path string) (*PiRuntimeHost, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("Pi runtime plan path is invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodePiRuntimeHost(data)
}

// AppendPiRuntimeProfile is the sole mutation of a Host runtime plan. A
// profile ref is write-once, and replacement is atomic under one Host lease.
func AppendPiRuntimeProfile(path string, profile PiRuntimeProfile) error {
	if !filepath.IsAbs(path) || profile.Validate() != nil {
		return errors.New("Pi runtime profile update is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("Pi runtime plan is unavailable")
	}
	lease, err := transaction.Acquire(filepath.Join(filepath.Dir(path), ".pi-runtime-plan.lock"))
	if err != nil {
		return err
	}
	defer lease.Release()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	profiles, err := decodePiRuntimeProfiles(data)
	if err != nil {
		return err
	}
	for _, existing := range profiles {
		if existing.Ref == profile.Ref {
			if !samePiRuntimeProfile(existing, profile) {
				return errors.New("Pi runtime profile identity changed")
			}
			return nil
		}
	}
	profiles = append(profiles, profile)
	encoded, err := EncodePiRuntimePlan(profiles)
	if err != nil {
		return err
	}
	return transaction.Replace(path, encoded, 0o600)
}

func samePiRuntimeProfile(left, right PiRuntimeProfile) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}
