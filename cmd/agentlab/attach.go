package main

import (
	"errors"
	"flag"
	"os"
	"time"

	piadapter "github.com/yansircc/agentlab/internal/adapter/pi"
	"github.com/yansircc/agentlab/internal/processidentity"
	"github.com/yansircc/agentlab/internal/run"
)

func attach(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab run attach <begin|poll>")
	}
	switch args[0] {
	case "begin":
		return attachBegin(args[1:])
	case "poll":
		return attachPoll(args[1:])
	default:
		return nil, errors.New("attach action must be begin or poll")
	}
}

func attachBegin(args []string) (any, error) {
	set := flag.NewFlagSet("run attach begin", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id")
	runID := set.String("run", "", "run id")
	adapter := set.String("adapter", "", "runtime adapter")
	stream := set.String("stream", "", "adapter stream path")
	pid := set.Int("pid", 0, "optional Pi worker pid")
	first := set.Duration("first-event", 30*time.Second, "first event timeout")
	soft := set.Duration("soft-idle", 2*time.Minute, "soft idle timeout")
	hard := set.Duration("hard-idle", 10*time.Minute, "hard idle timeout")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *experimentID == "" || *runID == "" || *adapter == "" || *stream == "" {
		return nil, errors.New("experiment id, run id, adapter, and stream path are required")
	}
	var identity *processidentity.Identity
	if *pid > 0 {
		captured, err := processidentity.CaptureProcess(*pid)
		if err != nil {
			return nil, err
		}
		identity = &captured
	}
	op, err := run.Open(*root, *experimentID, *runID)
	if err != nil {
		return nil, err
	}
	if *adapter != "pi" {
		return nil, errors.New("unknown attached runtime adapter")
	}
	return piadapter.Begin(op, *stream, run.StopPolicy{FirstEventTimeout: *first, SoftIdleTimeout: *soft, HardIdleTimeout: *hard}, identity)
}

func attachPoll(args []string) (any, error) {
	set := flag.NewFlagSet("run attach poll", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id")
	runID := set.String("run", "", "run id")
	adapter := set.String("adapter", "", "runtime adapter")
	stream := set.String("stream", "", "adapter stream path")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *experimentID == "" || *runID == "" || *adapter == "" || *stream == "" {
		return nil, errors.New("experiment id, run id, adapter, and stream path are required")
	}
	op, err := run.Open(*root, *experimentID, *runID)
	if err != nil {
		return nil, err
	}
	if *adapter != "pi" {
		return nil, errors.New("unknown attached runtime adapter")
	}
	return piadapter.Poll(op, *stream)
}
