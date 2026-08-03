package experiment

import "errors"

var (
	ErrInvalidID            = errors.New("invalid experiment id")
	ErrNotBegun             = errors.New("experiment has not begun")
	ErrAlreadyBegun         = errors.New("experiment already begun with different preparation")
	ErrPreparationNotSealed = errors.New("experiment preparation is not sealed")
)
