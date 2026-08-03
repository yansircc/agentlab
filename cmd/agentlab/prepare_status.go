package main

import (
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/preparation"
)

func parsePreparationTarget(name string, args []string) (*preparation.Operation, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	id := set.String("preparation", "", "preparation id")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *id == "" {
		return nil, errors.New("preparation id is required")
	}
	return preparation.Open(*root, *id)
}

func prepareStatus(args []string) (any, error) {
	op, err := parsePreparationTarget("prepare status", args)
	if err != nil {
		return nil, err
	}
	return op.Status()
}

func prepareChallengeBasis(args []string) (any, error) {
	op, err := parsePreparationTarget("prepare challenge-basis", args)
	if err != nil {
		return nil, err
	}
	return op.ChallengeBasis()
}

func prepareSeal(args []string) (any, error) {
	op, err := parsePreparationTarget("prepare seal", args)
	if err != nil {
		return nil, err
	}
	return op.Seal()
}
