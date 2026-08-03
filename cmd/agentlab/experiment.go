package main

import (
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/experiment"
)

func experimentCommand(args []string) (any, error) {
	if len(args) > 0 && args[0] == "bind-run" {
		return experimentBindRun(args[1:])
	}
	if len(args) == 0 || (args[0] != "begin" && args[0] != "status") {
		return nil, errors.New("usage: agentlab experiment <begin|bind-run|status>")
	}
	set := flag.NewFlagSet("experiment "+args[0], flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	id := set.String("experiment", "", "experiment id")
	preparationID := set.String("preparation", "", "sealed preparation id")
	if err := set.Parse(args[1:]); err != nil {
		return nil, err
	}
	if *id == "" {
		return nil, errors.New("experiment id is required")
	}
	op, err := experiment.Open(*root, *id)
	if err != nil {
		return nil, err
	}
	if args[0] == "status" {
		if *preparationID != "" {
			return nil, errors.New("preparation is only valid for experiment begin")
		}
		return op.Status()
	}
	if *preparationID == "" {
		return nil, errors.New("preparation id is required")
	}
	return op.Begin(*preparationID)
}
