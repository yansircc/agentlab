package filegit

import (
	"errors"
	"path/filepath"
	"sort"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/oracle"
)

func Execute(store artifact.Store, spec Spec) (Result, error) {
	if err := validateSpec(spec); err != nil {
		return Result{}, err
	}
	spec.Paths = append([]string(nil), spec.Paths...)
	sort.Strings(spec.Paths)
	files, err := captureFiles(store, spec.Root, spec.Paths, spec.MaxFileBytes)
	if err != nil {
		return Result{}, err
	}
	output := Output{Files: files}
	if spec.CaptureGit {
		git, err := captureGit(store, spec)
		if err != nil {
			return Result{}, err
		}
		output.Git = &git
	}
	receipt, err := oracle.Record(store, "file_git", spec, output, spec.SideEffects)
	if err != nil {
		return Result{}, err
	}
	return Result{Receipt: receipt, Output: output}, nil
}

func validateSpec(spec Spec) error {
	if !filepath.IsAbs(spec.Root) || len(spec.Paths) == 0 || spec.MaxFileBytes < 1 || spec.MaxFileBytes > 64*1024*1024 || len(spec.SideEffects) == 0 {
		return errors.New("absolute root, paths, bounded file size, and side effects are required")
	}
	if spec.CaptureGit && (!filepath.IsAbs(spec.GitExecutable) || spec.MaxGitBytes < 1 || spec.MaxGitBytes > 8*1024*1024) {
		return errors.New("git capture requires absolute executable and bounded output")
	}
	return nil
}
