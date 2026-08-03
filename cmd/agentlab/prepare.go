package main

import "errors"

func prepare(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: agentlab prepare <begin|record-fact|propose-decision|resolve|assay|challenge-basis|challenge|seal|status>")
	}
	switch args[0] {
	case "begin":
		return prepareBegin(args[1:])
	case "record-fact":
		return prepareRecordFact(args[1:])
	case "propose-decision":
		return preparePropose(args[1:])
	case "resolve":
		return prepareResolve(args[1:])
	case "assay":
		return prepareAssay(args[1:])
	case "challenge-basis":
		return prepareChallengeBasis(args[1:])
	case "challenge":
		return prepareChallenge(args[1:])
	case "seal":
		return prepareSeal(args[1:])
	case "status":
		return prepareStatus(args[1:])
	default:
		return nil, errors.New("unknown prepare command")
	}
}
