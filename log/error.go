package log

import (
	"errors"
	"fmt"
)

var ErrNilDriver = errors.New("nil log driver")

var ErrNilHandler = errors.New("nil log handler")

var ErrZeroHandler = errors.New("uninitialized log handler")

var ErrNilWriter = errors.New("nil log writer")

var ErrInvalidLevel = errors.New("invalid log level")

var ErrInvalidFormat = errors.New("invalid log format")

var ErrInvalidHandler = errors.New("invalid log handler")

// errf is a helper that appends a formatted message to a wrapped error.
func errf(err error, template string, args ...any) error {
	template = "%w: " + template
	args = append([]any{err}, args...)
	return fmt.Errorf(template, args...)
}
