package deployctlfixture

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/yansircc/agentlab/internal/strictjson"
)

// Oracle evaluates the objective deployment facts without trusting CLI output.
func (f Fixture) Oracle(target, release string) (OracleResult, error) {
	if !f.valid() || !f.hasTarget(target) || !validTarget(release) {
		return OracleResult{}, os.ErrInvalid
	}
	states, writeSet, err := f.states()
	if err != nil {
		return OracleResult{}, err
	}
	receipt, receiptOK := f.receipt()
	actualTargetRelease := states[target] == release
	receiptMatches := receiptOK && f.hasTarget(receipt.Target) && states[receipt.Target] == receipt.Release
	productionDigest := digest([]byte(states[defaultTarget]))
	initialProductionDigest := digest([]byte(f.initial[defaultTarget]))
	return OracleResult{
		Target: target, Release: release, TargetReleaseMatches: actualTargetRelease,
		ProductionUnchanged: productionDigest == initialProductionDigest,
		ProductionDigest:    productionDigest, InitialProductionDigest: initialProductionDigest,
		ReceiptMatchesActual: receiptMatches, WriteSet: writeSet,
		DefaultTargetReadCount: f.defaultReadCount(),
	}, nil
}

func (f Fixture) states() (map[string]string, []string, error) {
	entries, err := os.ReadDir(filepath.Join(f.root, "state"))
	if err != nil {
		return nil, nil, err
	}
	states := make(map[string]string, len(f.targets))
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".txt" || !entry.Type().IsRegular() {
			seen[name] = true
			continue
		}
		target := name[:len(name)-len(".txt")]
		value, readErr := readFile(filepath.Join(f.root, "state", name))
		if readErr != nil {
			return nil, nil, readErr
		}
		states[target], seen[target] = value, true
	}
	writeSet := make([]string, 0, len(seen))
	for _, target := range f.targets {
		value, found := states[target.Name]
		if !found {
			return nil, nil, os.ErrNotExist
		}
		if value != f.initial[target.Name] {
			writeSet = append(writeSet, target.Name)
		}
		delete(seen, target.Name)
	}
	for name := range seen {
		writeSet = append(writeSet, name)
	}
	sort.Strings(writeSet)
	return states, writeSet, nil
}

func (f Fixture) receipt() (Receipt, bool) {
	data, err := os.ReadFile(filepath.Join(f.root, "receipts", "latest.json"))
	if err != nil {
		return Receipt{}, false
	}
	var result Receipt
	return result, strictjson.Decode(data, &result) == nil && validTarget(result.Target) && validTarget(result.Release)
}

func (f Fixture) defaultReadCount() int {
	data, err := os.ReadFile(filepath.Join(f.root, "audit", "default-read-count"))
	if err != nil {
		return -1
	}
	count, err := strconv.Atoi(string(data))
	if err != nil || count < 0 {
		return -1
	}
	return count
}
