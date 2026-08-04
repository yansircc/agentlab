package deployctlfixture

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/strictjson"
)

// OpenFixture rebuilds the task-owned fixture projection from its stable
// catalog. Mutable deployment state is deliberately not read as a baseline.
func OpenFixture(root string) (Fixture, error) {
	info, err := os.Lstat(root)
	if !filepath.IsAbs(root) || err != nil || !info.IsDir() {
		return Fixture{}, errors.New("deployctl fixture root is invalid")
	}
	catalogPath := filepath.Join(root, "catalog.json")
	if fileIsRegular(catalogPath) != nil || fileIsRegular(filepath.Join(root, "default-target")) != nil {
		return Fixture{}, errors.New("deployctl fixture files are invalid")
	}
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return Fixture{}, err
	}
	var catalog struct {
		Targets []Target `json:"targets"`
	}
	if strictjson.Decode(data, &catalog) != nil || validateTargets(catalog.Targets) != nil {
		return Fixture{}, errors.New("deployctl fixture catalog is invalid")
	}
	defaultData, err := os.ReadFile(filepath.Join(root, "default-target"))
	if err != nil || string(defaultData) != defaultTarget {
		return Fixture{}, errors.New("deployctl fixture default target is invalid")
	}
	initial := make(map[string]string, len(catalog.Targets))
	for _, target := range catalog.Targets {
		initial[target.Name] = initialRelease
	}
	return Fixture{root: root, targets: catalog.Targets, initial: initial}, nil
}
