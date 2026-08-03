package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/yansircc/agentlab/internal/experiment"
	"github.com/yansircc/agentlab/internal/preparation"
	"github.com/yansircc/agentlab/internal/run"
)

func inspect(args []string) (any, error) {
	set := flag.NewFlagSet("inspect", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id for run inspection")
	runID := set.String("run", "", "run id")
	preparationID := set.String("preparation", "", "preparation id")
	afterText := set.String("after", "", "exclusive sequence cursor")
	limitText := set.String("limit", "", "record count, 1..1000")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *afterText == "" || *limitText == "" || (*preparationID != "" && (*runID != "" || *experimentID != "")) || (*preparationID == "" && *experimentID == "") {
		return nil, errors.New("preparation, experiment, or experiment-scoped run target plus after and limit is required")
	}
	after, err := strconv.ParseUint(*afterText, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid after: %w", err)
	}
	limit, err := strconv.Atoi(*limitText)
	if err != nil {
		return nil, fmt.Errorf("invalid limit: %w", err)
	}
	if *preparationID != "" {
		op, err := preparation.Open(*root, *preparationID)
		if err != nil {
			return nil, err
		}
		return op.Inspect(after, limit)
	}
	if *runID == "" {
		op, err := experiment.Open(*root, *experimentID)
		if err != nil {
			return nil, err
		}
		return op.Inspect(after, limit)
	}
	op, err := run.Open(*root, *experimentID, *runID)
	if err != nil {
		return nil, err
	}
	return op.Inspect(after, limit)
}
