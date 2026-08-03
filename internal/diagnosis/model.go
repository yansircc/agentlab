package diagnosis

import (
	"errors"
	"path"
	"regexp"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type State string

const (
	Hypothetical State = "hypothetical"
	Established  State = "established"
)

type SourceEvidenceRef struct {
	Artifact         artifact.Ref `json:"artifact"`
	Path             string       `json:"path"`
	StartLine        int          `json:"start_line"`
	EndLine          int          `json:"end_line"`
	EstablishesOwner bool         `json:"establishes_owner"`
}

type Claim struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Falsifier string `json:"falsifier"`
}

type Diagnosis struct {
	ID                string              `json:"id"`
	State             State               `json:"state"`
	FindingIDs        []string            `json:"finding_ids"`
	SourceSnapshot    artifact.Ref        `json:"source_snapshot"`
	SourceEvidence    []SourceEvidenceRef `json:"source_evidence"`
	Owner             string              `json:"owner"`
	RootCause         string              `json:"root_cause"`
	Invariant         string              `json:"invariant"`
	RepairBoundary    string              `json:"repair_boundary"`
	ProhibitedPatches []string            `json:"prohibited_patches"`
	AcceptanceClaims  []Claim             `json:"acceptance_claims"`
}

func (d Diagnosis) Validate() error {
	if !idPattern.MatchString(d.ID) || len(d.FindingIDs) == 0 || len(d.FindingIDs) > 50 || !validRef(d.SourceSnapshot) {
		return errors.New("diagnosis identity, findings, or source snapshot is invalid")
	}
	if d.State != Hypothetical && d.State != Established {
		return errors.New("diagnosis state is invalid")
	}
	if d.Owner == "" || d.RootCause == "" || d.Invariant == "" || d.RepairBoundary == "" || len(d.SourceEvidence) == 0 || len(d.AcceptanceClaims) == 0 {
		return errors.New("diagnosis requires source-backed owner, invariant, repair boundary, and claims")
	}
	if d.State == Established && !hasOwnerEvidence(d.SourceEvidence) {
		return errors.New("established diagnosis requires explicit owner evidence")
	}
	if len(d.SourceEvidence) > 100 || len(d.AcceptanceClaims) > 100 || len(d.ProhibitedPatches) > 100 {
		return errors.New("diagnosis collections exceed bounds")
	}
	if err := validateEvidence(d.SourceEvidence); err != nil {
		return err
	}
	return validateTextAndIDs(d)
}

func validateEvidence(refs []SourceEvidenceRef) error {
	seen := map[SourceEvidenceRef]bool{}
	for _, ref := range refs {
		clean := path.Clean(ref.Path)
		if !validRef(ref.Artifact) || ref.Path == "" || path.IsAbs(ref.Path) || strings.Contains(ref.Path, `\`) || clean != ref.Path || clean == ".." || strings.HasPrefix(clean, "../") || ref.StartLine < 1 || ref.EndLine < ref.StartLine || seen[ref] {
			return errors.New("source evidence reference is invalid")
		}
		seen[ref] = true
	}
	return nil
}

func hasOwnerEvidence(refs []SourceEvidenceRef) bool {
	for _, ref := range refs {
		if ref.EstablishesOwner {
			return true
		}
	}
	return false
}

func validateTextAndIDs(d Diagnosis) error {
	seen := map[string]bool{}
	for _, id := range d.FindingIDs {
		if !idPattern.MatchString(id) || seen[id] {
			return errors.New("finding ids are invalid or duplicated")
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, claim := range d.AcceptanceClaims {
		if !idPattern.MatchString(claim.ID) || claim.Statement == "" || claim.Falsifier == "" || len(claim.Statement) > 4096 || len(claim.Falsifier) > 4096 || seen[claim.ID] {
			return errors.New("acceptance claim is invalid or duplicated")
		}
		seen[claim.ID] = true
	}
	for _, value := range append(append([]string{d.Owner, d.RootCause, d.Invariant, d.RepairBoundary}, d.ProhibitedPatches...), d.FindingIDs...) {
		if len(value) > 4096 {
			return errors.New("diagnosis text exceeds bounds")
		}
	}
	return nil
}

func validRef(ref artifact.Ref) bool {
	return ref.Algorithm == "sha256" && len(ref.Digest) == 64 && ref.Size >= 0
}
