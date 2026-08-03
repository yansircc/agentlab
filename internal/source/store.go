package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"

	"github.com/yansircc/agentlab/internal/artifact"
)

func Build(store artifact.Store, inputs []InputFile) (artifact.Ref, error) {
	if len(inputs) == 0 || len(inputs) > 10000 {
		return artifact.Ref{}, errors.New("source snapshot requires bounded files")
	}
	files := make([]File, 0, len(inputs))
	for _, input := range inputs {
		if !validPath(input.Path) {
			return artifact.Ref{}, errors.New("source input path is invalid")
		}
		ref, err := store.Put(input.Content)
		if err != nil {
			return artifact.Ref{}, err
		}
		files = append(files, File{Path: input.Path, Artifact: ref})
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	snapshot := Snapshot{Contract: Contract, Files: files}
	if err := snapshot.Validate(); err != nil {
		return artifact.Ref{}, err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.Put(data)
}

func Load(store artifact.Store, ref artifact.Ref) (Snapshot, error) {
	data, err := store.Read(ref)
	if err != nil {
		return Snapshot{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("source snapshot has trailing input")
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	for _, file := range snapshot.Files {
		if _, err := store.Read(file.Artifact); err != nil {
			return Snapshot{}, err
		}
	}
	return snapshot, nil
}

func (snapshot Snapshot) Contains(path string, ref artifact.Ref) bool {
	index := sort.Search(len(snapshot.Files), func(index int) bool { return snapshot.Files[index].Path >= path })
	return index < len(snapshot.Files) && snapshot.Files[index].Path == path && snapshot.Files[index].Artifact == ref
}
