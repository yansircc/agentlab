package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/yansircc/agentlab/internal/run"
)

type startRequest struct {
	PublicCommand            []string          `json:"public_command"`
	PublicEnvironment        map[string]string `json:"public_environment,omitempty"`
	SecretEnvironmentHandles map[string]string `json:"secret_environment_handles,omitempty"`
}

func start(args []string) (any, error) {
	set := flag.NewFlagSet("run start", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	experimentID := set.String("experiment", "", "experiment id")
	runID := set.String("run", "", "run id")
	requestPath := set.String("request", "-", "JSON request path or - for stdin")
	first := set.Duration("first-event", 0, "first event timeout")
	soft := set.Duration("soft-idle", 0, "soft idle timeout")
	hard := set.Duration("hard-idle", 0, "hard idle timeout")
	kill := set.Bool("kill-on-hard-idle", false, "terminate owned worker after hard idle")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	var request startRequest
	if err := readRequest(*requestPath, &request); err != nil {
		return nil, err
	}
	if *experimentID == "" || *runID == "" || len(request.PublicCommand) == 0 || set.NArg() != 0 {
		return nil, errors.New("experiment id, run id, and public command are required")
	}
	op, err := run.Open(*root, *experimentID, *runID)
	if err != nil {
		return nil, err
	}
	return op.Start(context.Background(), *runID, run.StartSpec{
		PublicCommand: request.PublicCommand, PublicEnvironment: request.PublicEnvironment,
		SecretEnvironmentHandles: request.SecretEnvironmentHandles,
		Policy:                   run.StopPolicy{FirstEventTimeout: *first, SoftIdleTimeout: *soft, HardIdleTimeout: *hard, OwnsWorkerProcess: true, KillOnHardIdle: *kill},
	})
}
