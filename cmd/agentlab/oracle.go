package main

import "errors"

func oracleCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab oracle <command|http|file-git>")
	}
	switch args[0] {
	case "command":
		return runCommandOracle(args[1:])
	case "http":
		return runHTTPOracle(args[1:])
	case "file-git":
		return runFileGitOracle(args[1:])
	default:
		return nil, errors.New("unknown oracle command")
	}
}
