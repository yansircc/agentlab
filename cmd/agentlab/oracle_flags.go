package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/yansircc/agentlab/internal/artifact"
)

type oracleFlags struct {
	request string
	store   artifact.Store
}

func parseOracleFlags(name string, args []string) (oracleFlags, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	request := set.String("request", "-", "JSON request path or - for stdin")
	if err := set.Parse(args); err != nil {
		return oracleFlags{}, err
	}
	return oracleFlags{request: *request, store: artifact.NewStore(filepath.Join(*root, "artifacts"))}, nil
}
