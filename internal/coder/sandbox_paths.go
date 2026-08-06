package coder

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func sandboxDirectory(value string) (string, error) {
	if !filepath.IsAbs(value) || temporary(value) || !profilePath(value) {
		return "", errors.New("coder sandbox path is invalid")
	}
	path, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() || !profilePath(path) {
		return "", errors.New("coder sandbox path is not a directory")
	}
	return path, nil
}

func sandboxDirectories(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		root, err := sandboxDirectory(value)
		if err != nil || seen[root] {
			return nil, errors.New("coder sandbox read root is invalid")
		}
		seen[root] = true
		result = append(result, root)
	}
	sort.Strings(result)
	return result, nil
}

func sandboxExecutables(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if !filepath.IsAbs(value) || temporary(value) || !profilePath(value) {
			return nil, errors.New("coder sandbox executable is invalid")
		}
		path, err := filepath.EvalSymlinks(value)
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || !profilePath(path) || seen[path] {
			return nil, errors.New("coder sandbox executable is invalid")
		}
		seen[path] = true
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

func overlaps(left, right string) bool {
	return contains(left, right) || contains(right, left)
}

func contains(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && (relative == "." || (!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != ".."))
}

func temporary(path string) bool {
	return path == "/tmp" || strings.HasPrefix(path, "/tmp/") || path == "/private/tmp" || strings.HasPrefix(path, "/private/tmp/")
}

func profilePath(path string) bool { return !strings.ContainsAny(path, "\"\r\n()") }
