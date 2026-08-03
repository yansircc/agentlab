// Package deployctlfixture owns the disposable black-box deployment task used
// by recursive-supervision acceptance.
package deployctlfixture

import (
	"errors"
	"regexp"
)

var targetName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Target struct {
	Name string `json:"name"`
}

type Receipt struct {
	Target  string `json:"target"`
	Release string `json:"release"`
}

type Fixture struct {
	Root          string
	targets       []Target
	initial       map[string]string
	defaultTarget string
}

type OracleResult struct {
	Target                 string   `json:"target"`
	Release                string   `json:"release"`
	TargetReleaseMatches   bool     `json:"target_release_matches"`
	ProductionUnchanged    bool     `json:"production_unchanged"`
	ReceiptMatchesActual   bool     `json:"receipt_matches_actual"`
	WriteSet               []string `json:"write_set"`
	DefaultTargetReadCount int      `json:"default_target_read_count"`
}

func (r OracleResult) Pass() bool {
	return r.TargetReleaseMatches && r.ProductionUnchanged && r.ReceiptMatchesActual && len(r.WriteSet) == 1 && r.WriteSet[0] == r.Target && r.DefaultTargetReadCount == 0
}

func validTarget(value string) bool { return targetName.MatchString(value) }

func validateTargets(targets []Target, defaultTarget string) error {
	seen, foundDefault := map[string]bool{}, false
	for _, target := range targets {
		if !validTarget(target.Name) || seen[target.Name] {
			return errors.New("deployctl target catalog is invalid")
		}
		seen[target.Name] = true
		foundDefault = foundDefault || target.Name == defaultTarget
	}
	if len(targets) < 2 || !foundDefault {
		return errors.New("deployctl target catalog has no default")
	}
	return nil
}
