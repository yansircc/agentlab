package preparation

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/strictjson"
)

// RenderWorkerInput is the only conversion from a sealed WorkerInput to the
// bytes placed in a Worker prompt. It follows refs already sealed by
// preparation and cannot import any additional task material.
func RenderWorkerInput(store artifact.Store, ref artifact.Ref) (string, error) {
	if !ref.Valid() {
		return "", errors.New("worker input ref is invalid")
	}
	data, err := store.Read(ref)
	if err != nil {
		return "", err
	}
	var input WorkerInput
	if strictjson.Decode(data, &input) != nil || !validWorkerInput(input) {
		return "", errors.New("worker input is invalid")
	}
	refs := append([]artifact.Ref{input.UserIntentRef}, input.PublicArtifacts...)
	parts := make([]string, 0, len(refs))
	total := 0
	for _, item := range refs {
		value, readErr := store.Read(item)
		if readErr != nil || len(value) == 0 || !utf8.Valid(value) || total+len(value) > 65536 {
			return "", errors.New("worker input material is invalid")
		}
		total += len(value)
		parts = append(parts, string(value))
	}
	return strings.Join(parts, "\n\n"), nil
}

func validWorkerInput(value WorkerInput) bool {
	if value.Contract != workerInputContract || !value.UserIntentRef.Valid() || len(value.PublicArtifacts) > 100 {
		return false
	}
	seen := map[artifact.Ref]bool{value.UserIntentRef: true}
	for _, ref := range value.PublicArtifacts {
		if !ref.Valid() || seen[ref] {
			return false
		}
		seen[ref] = true
	}
	return true
}
