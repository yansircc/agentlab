package main

import "errors"

func diagnoseCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab diagnose <record|bind-candidate>")
	}
	switch args[0] {
	case "record":
		return diagnoseRecord(args[1:])
	case "bind-candidate":
		return diagnoseBindCandidate(args[1:])
	default:
		return nil, errors.New("unknown diagnose command")
	}
}
