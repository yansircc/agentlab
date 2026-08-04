package tool

import (
	"errors"
	"unicode/utf8"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/effect"
	"github.com/yansircc/agentlab/internal/run"
)

const (
	runtimeTreePageContract = "agentlab.pi-runtime-tree-page.v2"
	maxRuntimeTreePage      = 100
	maxRuntimeTreeText      = 8192
)

// PiRuntimeTreePage is a bounded, public-only projection of one Host-bound
// Worker session. RuntimeRef and entry locators are opaque handles; neither a
// session path nor a raw Pi record can cross the provider boundary.
type PiRuntimeTreePage struct {
	Contract       string               `json:"contract"`
	RunID          string               `json:"run_id"`
	RuntimeRef     string               `json:"runtime_ref"`
	RuntimeLocator string               `json:"runtime_locator"`
	After          uint64               `json:"after"`
	NextAfter      *uint64              `json:"next_after,omitempty"`
	Entries        []PiRuntimeTreeEntry `json:"entries"`
}

type PiRuntimeTreeEntry struct {
	Evidence             run.EvidenceRef `json:"evidence_ref"`
	Locator              string          `json:"locator"`
	ParentLocator        string          `json:"parent_locator,omitempty"`
	Role                 string          `json:"role"`
	Kind                 string          `json:"kind"`
	PublicText           string          `json:"public_text"`
	PublicTextTruncated  bool            `json:"public_text_truncated,omitempty"`
	PrefixDigest         string          `json:"prefix_digest"`
	StructurallyForkable bool            `json:"structurally_forkable"`
}

// RuntimeTree reads the adapter-owned session selected by a Host profile. A
// provider supplies only the run id and pagination cursor; a profile ref is
// returned only after the Host has resolved the run unambiguously.
func (h *PiRuntimeHost) RuntimeTree(binding Binding, runID string, after uint64, limit int) (any, error) {
	if h == nil || runID == "" || limit < 1 || limit > maxRuntimeTreePage {
		return nil, errors.New("Pi runtime tree request is invalid")
	}
	ref, profile, err := h.workerProfileForRun(binding, runID)
	if err != nil {
		return nil, errors.New("Pi Worker runtime tree is unavailable")
	}
	tree, err := piadapter.ReadPublicTree(profile.SessionPath)
	if err != nil || after > uint64(len(tree.Entries)) {
		return nil, errors.New("Pi Worker runtime tree is unavailable")
	}
	end := int(after) + limit
	if end > len(tree.Entries) {
		end = len(tree.Entries)
	}
	page := PiRuntimeTreePage{
		Contract: runtimeTreePageContract, RunID: runID, RuntimeRef: ref,
		RuntimeLocator: tree.RuntimeLocator, After: after,
		Entries: make([]PiRuntimeTreeEntry, 0, end-int(after)),
	}
	for _, entry := range tree.Entries[after:end] {
		evidence, err := h.runtimeTreeEvidence(binding, runID, entry)
		if err != nil {
			return nil, errors.New("Pi Worker runtime tree evidence is unavailable")
		}
		text, truncated := boundedRuntimeTreeText(entry.PublicText)
		page.Entries = append(page.Entries, PiRuntimeTreeEntry{
			Evidence: evidence, Locator: entry.Locator, ParentLocator: entry.ParentLocator, Role: entry.Role, Kind: entry.Kind,
			PublicText: text, PublicTextTruncated: truncated, PrefixDigest: entry.PrefixDigest,
			StructurallyForkable: entry.StructurallyForkable,
		})
	}
	if end < len(tree.Entries) {
		next := uint64(end)
		page.NextAfter = &next
	}
	return page, nil
}

func (h *PiRuntimeHost) runtimeTreeEvidence(binding Binding, runID string, entry piadapter.PublicEntry) (run.EvidenceRef, error) {
	source, err := entry.EvidenceSource()
	if err != nil {
		return run.EvidenceRef{}, err
	}
	experimentOp, err := binding.experiment()
	if err != nil {
		return run.EvidenceRef{}, err
	}
	for current := runID; current != ""; {
		op, err := run.Open(binding.Root, binding.ExperimentID, current)
		if err != nil {
			return run.EvidenceRef{}, err
		}
		evidence, err := op.EvidenceForSourceLocator(source)
		if err == nil {
			return evidence, nil
		}
		if !errors.Is(err, run.ErrEvidenceSourceAbsent) {
			return run.EvidenceRef{}, err
		}
		manifest, _, err := experimentOp.RunManifest(current)
		if err != nil {
			return run.EvidenceRef{}, err
		}
		splice, ok := manifest.Origin.Splice()
		if !ok {
			return run.EvidenceRef{}, errors.New("runtime entry is not durably admitted")
		}
		current = splice.ParentRun
	}
	return run.EvidenceRef{}, errors.New("runtime entry is not durably admitted")
}

func (h *PiRuntimeHost) workerProfileForRun(binding Binding, runID string) (string, PiRuntimeProfile, error) {
	if h == nil || binding.ExperimentID == "" || runID == "" {
		return "", PiRuntimeProfile{}, errors.New("Pi Worker runtime is absent")
	}
	var ref string
	var profile PiRuntimeProfile
	for candidateRef, candidate := range h.profiles {
		if candidate.ExperimentID != binding.ExperimentID || candidate.RunID != runID || candidate.Role != effect.WorkerStart {
			continue
		}
		if ref != "" {
			return "", PiRuntimeProfile{}, errors.New("Pi Worker runtime is ambiguous")
		}
		ref, profile = candidateRef, candidate
	}
	for candidateRef, template := range h.preparedWorkers {
		if template.ExperimentID != binding.ExperimentID || template.RunID != runID {
			continue
		}
		if ref != "" {
			return "", PiRuntimeProfile{}, errors.New("Pi Worker runtime is ambiguous")
		}
		resolved, err := h.resolvePreparedWorker(binding, template)
		if err != nil {
			return "", PiRuntimeProfile{}, err
		}
		ref, profile = candidateRef, resolved
	}
	if ref == "" {
		return "", PiRuntimeProfile{}, errors.New("Pi Worker runtime is absent")
	}
	return ref, profile, nil
}

func boundedRuntimeTreeText(value string) (string, bool) {
	if len(value) <= maxRuntimeTreeText {
		return value, false
	}
	if !utf8.ValidString(value) {
		return "", true
	}
	limit := maxRuntimeTreeText
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	if limit == 0 {
		return "", true
	}
	return value[:limit], true
}
