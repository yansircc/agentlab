package main

import (
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/gate"
)

type gateRequest struct {
	ExperimentID string    `json:"experiment_id"`
	Gate         gate.Spec `json:"gate"`
}

func gateCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab gate <record|show>")
	}
	if args[0] == "record" {
		flags, err := parsePrepareRequest("gate record", args[1:])
		if err != nil {
			return nil, err
		}
		var request gateRequest
		if err := readRequest(flags.request, &request); err != nil {
			return nil, err
		}
		operation, err := experiment.Open(flags.root, request.ExperimentID)
		if err != nil {
			return nil, err
		}
		return operation.RecordGate(request.Gate)
	}
	if args[0] != "show" {
		return nil, errors.New("unknown gate command")
	}
	set := flag.NewFlagSet("gate show", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id")
	gateID := set.String("gate", "", "gate id")
	if err := set.Parse(args[1:]); err != nil {
		return nil, err
	}
	if *experimentID == "" || *gateID == "" || set.NArg() != 0 {
		return nil, errors.New("experiment and gate ids are required")
	}
	operation, err := experiment.Open(*root, *experimentID)
	if err != nil {
		return nil, err
	}
	return operation.Gate(*gateID)
}
