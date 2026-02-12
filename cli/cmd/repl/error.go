package repl

import "errors"

// Sentinel errors.
var (
	ErrNoSource     = errors.New("no source files provided")
	ErrOutOfBounds  = errors.New("index out of range")
	ErrEditDeclined = errors.New("user declined to re-edit")
)
