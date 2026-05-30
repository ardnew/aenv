package cli

// Error is a command-line error with a process exit code.
type Error struct {
// Err is the underlying error.
	Err  error
	// Code is the process exit code.
	Code int
}

// ExitCode returns the process exit code or status.
func (e Error) ExitCode() int { return e.Code }

// Error returns the error message.
func (e Error) Error() string { return e.Err.Error() }

// Unwrap returns the underlying error, if any.
func (e Error) Unwrap() error { return e.Err }

func withExitCode(err error, code int) error {
	if err == nil {
		return nil
	}
	return Error{Err: err, Code: code}
}
