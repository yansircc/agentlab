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
	host, err := DecodePiRuntimeHost(data)
	if err != nil {
		return nil, err
	}
	host.planPath = path
	host.hostOracle = newHostWorkerOracle(path)
	return host, nil
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
	plan, err := decodePiRuntimePlan(data)
	if err != nil {
		return err
	}
	for _, existing := range plan.Profiles {
		if existing.Ref == profile.Ref {
			if !samePiRuntimeProfile(existing, profile) {
				return errors.New("Pi runtime profile identity changed")
			}
			return nil
		}
	}
	plan.Profiles = append(plan.Profiles, profile)
	encoded, err := encodePiRuntimePlan(plan.Profiles, plan.PreparedWorkers)
	if err != nil {
		return err
	}
	return transaction.Replace(path, encoded, 0o600)
}

// AppendPiPreparedWorkerRuntime registers the complete Host-derived runtime
// template issued with one PreparedRun. Its ref is immutable and cannot be
// replaced with a static profile or another template.
func AppendPiPreparedWorkerRuntime(path string, profile PiPreparedWorkerRuntime) error {
	if !filepath.IsAbs(path) || profile.Validate() != nil {
		return errors.New("prepared Pi Worker runtime update is invalid")
	}
	return updatePiRuntimePlan(path, func(plan *piRuntimePlan) error {
		for _, existing := range plan.Profiles {
			if existing.Ref == profile.Ref {
				return errors.New("Pi runtime profile ref is already bound")
			}
		}
		for _, existing := range plan.PreparedWorkers {
			if existing.Ref != profile.Ref {
				continue
			}
			if !samePiPreparedWorkerRuntime(existing, profile) {
				return errors.New("prepared Pi Worker runtime identity changed")
			}
			return nil
		}
		plan.PreparedWorkers = append(plan.PreparedWorkers, profile)
		return nil
	})
}

// BindPiForkedWorkerRuntime is the only permitted transition for a prepared
// Worker template. It records one settled fork receipt; a retry may reproduce
// that exact binding but cannot substitute another child or receipt.
func BindPiForkedWorkerRuntime(path, ref string, forked PiForkedWorkerBinding) error {
	if !filepath.IsAbs(path) || ref == "" || forked.Validate() != nil {
		return errors.New("forked Pi Worker runtime update is invalid")
	}
	return updatePiRuntimePlan(path, func(plan *piRuntimePlan) error {
		for index := range plan.PreparedWorkers {
			existing := &plan.PreparedWorkers[index]
			if existing.Ref != ref {
				continue
			}
			if existing.Forked == nil {
				copy := forked
				existing.Forked = &copy
				return nil
			}
			if !samePiForkedWorkerBinding(*existing.Forked, forked) {
				return errors.New("forked Pi Worker runtime identity changed")
			}
			return nil
		}
		return errors.New("prepared Pi Worker runtime is absent")
	})
}

func updatePiRuntimePlan(path string, update func(*piRuntimePlan) error) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || update == nil {
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
	plan, err := decodePiRuntimePlan(data)
	if err != nil {
		return err
	}
	if err := update(&plan); err != nil {
		return err
	}
	encoded, err := encodePiRuntimePlan(plan.Profiles, plan.PreparedWorkers)
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

func samePiPreparedWorkerRuntime(left, right PiPreparedWorkerRuntime) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func samePiForkedWorkerBinding(left, right PiForkedWorkerBinding) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}
