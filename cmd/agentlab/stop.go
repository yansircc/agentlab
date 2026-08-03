package main

import (
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/run"
)

func stop(args []string) (any, error) {
	set := flag.NewFlagSet("run stop", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id")
	runID := set.String("run", "", "run id")
	reason := set.String("reason", "user_request", "stop reason")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *experimentID == "" || *runID == "" {
		return nil, errors.New("experiment id and run id are required")
	}
	op, err := run.Open(*root, *experimentID, *runID)
	if err != nil {
		return nil, err
	}
	return op.RequestStop(*reason)
}
