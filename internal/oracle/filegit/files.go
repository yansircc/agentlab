package filegit

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
)

func captureFiles(store artifact.Store, root string, paths []string, maxBytes int64) ([]FileFact, error) {
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer rootFS.Close()
	ordered := make([]string, 0, len(paths))
	for _, relative := range paths {
		clean := filepath.Clean(relative)
		if relative == "" || filepath.IsAbs(relative) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, errors.New("file paths must be unique root-contained relative paths")
		}
		ordered = append(ordered, clean)
	}
	sort.Strings(ordered)
	result := make([]FileFact, 0, len(ordered))
	for index, relative := range ordered {
		if index > 0 && relative == ordered[index-1] {
			return nil, errors.New("file paths must be unique root-contained relative paths")
		}
		result = append(result, captureFile(store, rootFS, relative, maxBytes))
	}
	return result, nil
}

func captureFile(store artifact.Store, root *os.Root, relative string, maxBytes int64) FileFact {
	fact := FileFact{Path: relative}
	info, err := root.Lstat(relative)
	if err != nil {
		fact.Kind, fact.Failure = "missing", err.Error()
		return fact
	}
	fact.Mode, fact.Size = uint32(info.Mode()), info.Size()
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		fact.Kind = "symlink"
		fact.LinkTarget, err = root.Readlink(relative)
	case info.IsDir():
		fact.Kind = "directory"
	case info.Mode().IsRegular():
		fact.Kind = "regular"
		fact = captureRegular(store, root, fact, maxBytes)
	default:
		fact.Kind = "special"
	}
	if err != nil {
		fact.Failure = err.Error()
	}
	return fact
}

func captureRegular(store artifact.Store, root *os.Root, fact FileFact, maxBytes int64) FileFact {
	file, err := root.Open(fact.Path)
	if err != nil {
		fact.Failure = err.Error()
		return fact
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		fact.Failure = "file identity changed during capture"
		return fact
	}
	fact.Mode, fact.Size = uint32(info.Mode()), info.Size()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		fact.Failure = err.Error()
		return fact
	}
	if int64(len(data)) > maxBytes {
		fact.Failure = "file exceeds declared capture bound"
		return fact
	}
	ref, err := store.Put(data)
	if err != nil {
		fact.Failure = err.Error()
		return fact
	}
	fact.Content = &ref
	return fact
}
