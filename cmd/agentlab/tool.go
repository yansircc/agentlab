package main

import (
	"errors"
	"flag"
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
	input := set.String("input", "", "tool input JSON file")
	if err := set.Parse(args); err != nil {
		return nil, err
	}
	if *name == "" || *input == "" || *input == "-" || set.NArg() != 0 {
		return nil, errors.New("tool invoke requires name and input file")
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return nil, err
	}
	invocation, err := tool.Decode(*name, data)
	if err != nil {
		return nil, err
	}
	return dispatch(invocation.Args)
}
