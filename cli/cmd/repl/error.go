package repl

import "errors"

// Sentinel errors.
var (
	ErrOutOfBounds  = errors.New("index out of range")
	ErrEditDeclined = errors.New("decline edit")
)
