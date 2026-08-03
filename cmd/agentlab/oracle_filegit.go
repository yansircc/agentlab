package main

import filegitoracle "github.com/yansircc/agentlab/internal/oracle/filegit"

func runFileGitOracle(args []string) (any, error) {
	flags, err := parseOracleFlags("oracle file-git", args)
	if err != nil {
		return nil, err
	}
	var spec filegitoracle.Spec
	if err := readRequest(flags.request, &spec); err != nil {
		return nil, err
	}
	return filegitoracle.Execute(flags.store, spec)
}
