package main

import (
	"errors"
	"flag"
	"io"
	"os"

	"github.com/yansircc/agentlab/internal/tool"
)

func toolCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab tool <schemas|invoke>")
	}
	switch args[0] {
	case "schemas":
		return toolSchemas(args[1:])
	case "invoke":
		return toolInvoke(args[1:])
	default:
		return nil, errors.New("unknown tool command")
	}
}

func toolSchemas(args []string) (any, error) {
	set := flag.NewFlagSet("tool schemas", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	provider := set.String("provider", "", "anthropic or openai_responses")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if set.NArg() != 0 {
		return nil, errors.New("tool schemas accepts no positional arguments")
	}
	return tool.Projection(*provider)
}

func toolInvoke(args []string) (any, error) {
	set := flag.NewFlagSet("tool invoke", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	name := set.String("name", "", "AgentLab tool name")
	root := set.String("root", "", "host-bound storage root")
	preparationID := set.String("preparation", "", "host-bound preparation id")
	experimentID := set.String("experiment", "", "host-bound experiment id")
	authority := set.String("authority", "supervisor", "host-bound authority profile")
	piRuntimePlan := set.String("pi-runtime-plan", "", "host-bound Pi runtime plan")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *name == "" || *root == "" || set.NArg() != 0 {
		return nil, errors.New("tool invoke requires host-bound name and root; input is stdin")
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return nil, err
	}
	operation, err := tool.Decode(*name, data)
	if err != nil {
		return nil, err
	}
	binding := tool.Binding{Root: *root, PreparationID: *preparationID, ExperimentID: *experimentID, Authority: *authority}
	if *piRuntimePlan != "" {
		plan, err := os.ReadFile(*piRuntimePlan)
		if err != nil {
			return nil, err
		}
		host, err := tool.DecodePiRuntimeHost(plan)
		if err != nil {
			return nil, err
		}
		binding.Runtime = host
	}
	return tool.Execute(binding, operation)
}
