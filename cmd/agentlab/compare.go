package main

import (
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/comparison"
	"github.com/yansircc/agentlab/internal/experiment"
)

type comparisonRequest struct {
	ExperimentID string                 `json:"experiment_id"`
	Observation  comparison.Observation `json:"observation"`
}

func compareCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab compare <record|show>")
	}
	if args[0] == "record" {
		flags, err := parsePrepareRequest("compare record", args[1:])
		if err != nil {
			return nil, err
		}
		var request comparisonRequest
		if err := readRequest(flags.request, &request); err != nil {
			return nil, err
		}
		op, err := experiment.Open(flags.root, request.ExperimentID)
		if err != nil {
			return nil, err
		}
		return op.Compare(request.Observation)
	}
	if args[0] != "show" {
		return nil, errors.New("unknown compare command")
	}
	set := flag.NewFlagSet("compare show", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id")
	comparisonID := set.String("comparison", "", "comparison id")
	if err := set.Parse(args[1:]); err != nil {
		return nil, err
	}
	if *experimentID == "" || *comparisonID == "" {
		return nil, errors.New("experiment and comparison ids are required")
	}
	op, err := experiment.Open(*root, *experimentID)
	if err != nil {
		return nil, err
	}
	return op.Comparison(*comparisonID)
}
