package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
)

func artifactCommand(args []string) (any, error) {
	if len(args) == 0 || args[0] != "put" {
		return nil, errors.New("usage: agentlab artifact put")
	}
	set := flag.NewFlagSet("artifact put", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	path := set.String("file", "", "artifact file")
	if err := set.Parse(args[1:]); err != nil {
		return nil, err
	}
	if *path == "" || *path == "-" {
		return nil, errors.New("artifact file path is required")
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		return nil, err
	}
	return artifact.NewStore(filepath.Join(*root, "artifacts")).Put(data)
}
