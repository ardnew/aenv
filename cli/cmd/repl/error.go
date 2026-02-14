package repl

import "errors"

// Sentinel errors.
var (
	ErrNoSource     = errors.New("require source files")
	ErrOutOfBounds  = errors.New("index out of range")
	ErrEditDeclined = errors.New("decline edit")
)
