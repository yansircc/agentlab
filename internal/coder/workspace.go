// Package coder owns the filesystem capability granted to one Coder session.
package coder

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/source"
	"github.com/yansircc/agentlab/internal/strictjson"
)

const workspaceContract = "agentlab.coder-workspace.v1"

type Receipt struct {
	Contract       string       `json:"contract"`
	SourceSnapshot artifact.Ref `json:"source_snapshot"`
}

type Workspace struct{ root string }

func Prepare(store artifact.Store, sourceRef artifact.Ref, root string) (artifact.Ref, error) {
	snapshot, err := source.Load(store, sourceRef)
	if err != nil || !filepath.IsAbs(root) || emptyDirectory(root) != nil {
		return artifact.Ref{}, errors.New("coder workspace is invalid")
	}
	for _, file := range snapshot.Files {
		data, err := store.Read(file.Artifact)
		if err != nil || write(root, file.Path, data) != nil {
			return artifact.Ref{}, errors.New("coder workspace materialization failed")
		}
	}
	encoded, err := json.Marshal(Receipt{Contract: workspaceContract, SourceSnapshot: sourceRef})
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.Put(encoded)
}

func Open(store artifact.Store, ref, sourceRef artifact.Ref, root string) (Workspace, error) {
	data, err := store.Read(ref)
	if err != nil {
		return Workspace{}, err
	}
	var receipt Receipt
	if strictjson.Decode(data, &receipt) != nil || receipt.Contract != workspaceContract || receipt.SourceSnapshot != sourceRef || !filepath.IsAbs(root) {
		return Workspace{}, errors.New("coder workspace receipt is invalid")
	}
	snapshot, err := source.Load(store, sourceRef)
	if err != nil || verify(store, snapshot, root) != nil {
		return Workspace{}, errors.New("coder workspace does not match source snapshot")
	}
	return Workspace{root: root}, nil
}

func (w Workspace) Read(name string) ([]byte, error) {
	file, err := securePath(w.root, name, false)
	if err != nil {
		return nil, err
	}
	handle, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	return io.ReadAll(io.LimitReader(handle, 64<<20))
}

func (w Workspace) Write(name string, data []byte) error {
	file, err := securePath(w.root, name, true)
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o600)
}

func (w Workspace) Seal(store artifact.Store) (artifact.Ref, error) {
	var inputs []source.InputFile
	err := filepath.WalkDir(w.root, func(current string, entry os.DirEntry, err error) error {
		if err != nil || current == w.root {
			return err
		}
		relative, err := filepath.Rel(w.root, current)
		if err != nil || !safe(relative) || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("coder workspace entry is invalid")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || len(inputs) == 10000 {
			return errors.New("coder workspace file is invalid")
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		inputs = append(inputs, source.InputFile{Path: filepath.ToSlash(relative), Content: data})
		return nil
	})
	if err != nil || len(inputs) == 0 {
		return artifact.Ref{}, errors.New("coder workspace cannot be sealed")
	}
	sort.Slice(inputs, func(left, right int) bool { return inputs[left].Path < inputs[right].Path })
	return source.Build(store, inputs)
}

func emptyDirectory(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return errors.New("workspace is not empty")
	}
	return nil
}

func verify(store artifact.Store, snapshot source.Snapshot, root string) error {
	files, err := workspaceFiles(root)
	if err != nil || len(files) != len(snapshot.Files) {
		return errors.New("workspace file set differs")
	}
	for index, file := range snapshot.Files {
		if files[index] != file.Path {
			return errors.New("workspace file set differs")
		}
		want, err := store.Read(file.Artifact)
		got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil || readErr != nil || !bytes.Equal(got, want) {
			return errors.New("workspace file differs")
		}
	}
	return nil
}

func workspaceFiles(root string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, err error) error {
		if err != nil || current == root {
			return err
		}
		relative, err := filepath.Rel(root, current)
		if err != nil || !safe(relative) || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("workspace entry is invalid")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || len(result) == 10000 {
			return errors.New("workspace file is invalid")
		}
		result = append(result, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(result)
	return result, err
}

func write(root, name string, data []byte) error {
	file, err := securePath(root, name, true)
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o600)
}
