package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
)

type manifestReceipt struct {
	RunID    string       `json:"run_id"`
	Manifest artifact.Ref `json:"manifest"`
}

func (o *Operation) requireManifest() (artifact.Ref, error) {
	data, err := os.ReadFile(filepath.Join(o.dir, "manifest.json"))
	if err != nil {
		return artifact.Ref{}, errors.New("run manifest binding is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt manifestReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return artifact.Ref{}, errors.New("run manifest binding is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || receipt.RunID != o.runID || !validRef(receipt.Manifest) {
		return artifact.Ref{}, errors.New("run manifest binding is invalid")
	}
	if _, err := o.artifacts.Read(receipt.Manifest); err != nil {
		return artifact.Ref{}, err
	}
	return receipt.Manifest, nil
}
