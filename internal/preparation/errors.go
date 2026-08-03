package preparation

import "errors"

var (
	ErrInvalidID            = errors.New("invalid preparation id")
	ErrNotBegun             = errors.New("preparation has not begun")
	ErrAlreadyBegun         = errors.New("preparation already begun")
	ErrSealed               = errors.New("preparation is sealed")
	ErrWrongFrontier        = errors.New("command does not target the current frontier")
	ErrUnresolved           = errors.New("preparation has unresolved decisions")
	ErrChallengeOpen        = errors.New("preparation challenge has open gaps")
	ErrChallengeNeeded      = errors.New("preparation requires a clean challenge")
	ErrLeakageAssayRequired = errors.New("preparation requires a clean leakage assay")
	ErrLeakageDetected      = errors.New("preparation worker input has detected semantic leakage")
)
