package cli

import "fmt"

// Error pairs an error with an exit code.
type Error struct {
	// Err is the wrapped error.
	Err error
	// Code is the exit code.
	Code int
}

// ExitCode returns the exit code.
func (e Error) ExitCode() int { return e.Code }

// Error returns the message.
func (e Error) Error() string { return e.Err.Error() }

// Unwrap returns the wrapped error.
func (e Error) Unwrap() error { return e.Err }

// withExitCode wraps an error with an exit code,
// or returns nil if the error is nil.
func withExitCode(err error, code int) error {
	if err == nil {
		return nil
	}
	return Error{Err: err, Code: code}
}

// errf is a helper that appends a formatted message to a wrapped error.
func errf(err error, template string, args ...any) error {
	template = "%w: " + template
	args = append([]any{err}, args...)
	return fmt.Errorf(template, args...)
}
