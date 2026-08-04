package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type output struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func main() {
	data, err := dispatch(os.Args[1:])
	result := output{OK: err == nil, Data: data}
	if err != nil {
		result.Error, result.Data = err.Error(), nil
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if err != nil {
		os.Exit(1)
	}
}

func dispatch(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab run <start|attach|status> | agentlab inspect")
	}
	if args[0] == "inspect" {
		return inspect(args[1:])
	}
	if args[0] == "prepare" {
		return prepare(args[1:])
	}
	if args[0] == "oracle" {
		return oracleCommand(args[1:])
	}
	if args[0] == "experiment" {
		return experimentCommand(args[1:])
	}
	if args[0] == "review" {
		return reviewCommand(args[1:])
	}
	if args[0] == "artifact" {
		return artifactCommand(args[1:])
	}
	if args[0] == "diagnose" {
		return diagnoseCommand(args[1:])
	}
	if args[0] == "compare" {
		return compareCommand(args[1:])
	}
	if args[0] == "tool" {
		return toolCommand(args[1:])
	}
	if args[0] == "gate" {
		return gateCommand(args[1:])
	}
	if args[0] == "acceptance" {
		return acceptanceCommand(args[1:])
	}
	if args[0] != "run" || len(args) < 2 {
		return nil, fmt.Errorf("unknown command %q", args[0])
	}
	switch args[1] {
	case "start":
		return start(args[2:])
	case "attach":
		return attach(args[2:])
	case "status":
		return status(args[2:])
	case "stop":
		return stop(args[2:])
	default:
		return nil, fmt.Errorf("unknown run command %q", args[1])
	}
}
