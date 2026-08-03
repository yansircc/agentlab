package main

import "errors"

func reviewCommand(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab review <record|detect-repeated|detect-bypass|disposition|handoff>")
	}
	switch args[0] {
	case "record":
		return reviewRecord(args[1:])
	case "detect-repeated":
		return reviewDetectRepeated(args[1:])
	case "detect-bypass":
		return reviewDetectBypass(args[1:])
	case "disposition":
		return reviewDisposition(args[1:])
	case "handoff":
		return reviewHandoff(args[1:])
	default:
		return nil, errors.New("unknown review command")
	}
}
