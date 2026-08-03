package deployctlfixture

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
)

const (
	resetContract   = "agentlab.deployctl-reset.v1"
	heldoutContract = "agentlab.deployctl-heldout.v1"
	initialRelease  = "release-old"
)

// New creates the owned, disposable baseline fixture. Its public catalog has
// staging and production; production is the private default target source.
func New(root string) (Fixture, error) {
	return newFixture(root, baselineTargets)
}

func (f Fixture) Root() string { return f.root }

func newFixture(root string, names []string) (Fixture, error) {
	if !filepath.IsAbs(root) || os.Mkdir(root, 0o700) != nil {
		return Fixture{}, errors.New("deployctl fixture root must be a new absolute directory")
	}
	targets, err := canonicalTargets(names)
	if err != nil {
		_ = os.Remove(root)
		return Fixture{}, err
	}
	initial := make(map[string]string, len(targets))
	for _, target := range targets {
		initial[target.Name] = initialRelease
	}
	return Fixture{root: root, targets: targets, initial: initial}, nil
}

// Reset restores precisely the owned fixture state and returns its immutable
// baseline receipt. No candidate or external deployment state is reachable.
func (f Fixture) Reset() (ResetReceipt, error) {
	if !f.valid() {
		return ResetReceipt{}, errors.New("deployctl fixture is invalid")
	}
	if err := f.resetFiles(); err != nil {
		return ResetReceipt{}, err
	}
	return ResetReceipt{Contract: resetContract, CatalogDigest: f.catalogDigest(), StateDigest: f.stateDigest(f.initial)}, nil
}

// Heldout creates a fresh fixture after an exact candidate snapshot is sealed.
// The nonce becomes part of the catalog only here, never in the baseline.
func (f Fixture) Heldout(root string, candidate artifact.Ref, nonce string) (Fixture, string, HeldoutReceipt, error) {
	name := "heldout-" + nonce
	if !f.valid() || !candidate.Valid() || !validTarget(name) || f.hasTarget(name) {
		return Fixture{}, "", HeldoutReceipt{}, errors.New("deployctl held-out fixture is invalid")
	}
	names := make([]string, 0, len(f.targets)+1)
	for _, target := range f.targets {
		names = append(names, target.Name)
	}
	names = append(names, name)
	heldout, err := newFixture(root, names)
	if err != nil {
		return Fixture{}, "", HeldoutReceipt{}, err
	}
	reset, err := heldout.Reset()
	if err != nil {
		return Fixture{}, "", HeldoutReceipt{}, err
	}
	receipt := HeldoutReceipt{Contract: heldoutContract, Candidate: candidate, Target: name, Reset: reset}
	return heldout, name, receipt, nil
}

func (f Fixture) valid() bool {
	if !filepath.IsAbs(f.root) || validateTargets(f.targets) != nil || len(f.initial) != len(f.targets) {
		return false
	}
	for _, target := range f.targets {
		if f.initial[target.Name] != initialRelease {
			return false
		}
	}
	return true
}
