package run

import "errors"

var (
	ErrInvalidPolicy       = errors.New("invalid stop policy")
	ErrInvalidRunID        = errors.New("invalid run id")
	ErrInvalidExperimentID = errors.New("invalid experiment id")
	ErrRunStarted          = errors.New("run already has durable events")
	ErrAttemptUnresolved   = errors.New("prior launch attempt is unresolved")
)
