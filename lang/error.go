package lang

import (
	"errors"
	"log/slog"
	"strings"
)

// Sentinel errors (structured errors with support for attributes).
var (
	ErrParse            = NewError("parse input")
	ErrExprCompile      = NewError("compile expression")
	ErrExprEvaluate     = NewError("evaluate expression")
	ErrNotDefined       = NewError("unknown identifier")
	ErrParameterCount   = NewError("wrong parameter count")
	ErrArgumentCount    = NewError("wrong argument count")
	ErrCycle            = NewError("reference cycle")
	ErrInvalidValueType = NewError("invalid value type")
	ErrValidation       = NewError("validation failed")
)

// Error is a structured error with optional attributes for structured logging.
// It implements both error and slog.LogValuer interfaces.
type Error struct {
	msg   string      // Base error message
	err   error       // Wrapped error (for errors.Unwrap)
	attrs []slog.Attr // Attributes for structured logging
}

// NewError creates a new Error with a message.
func NewError(msg string) *Error {
	return &Error{msg: msg}
}

// WrapError wraps a standard error into an Error.
func WrapError(err error) *Error {
	ee := &Error{}
	if errors.As(err, &ee) {
		return ee
	}

	return &Error{err: err}
}

// Error implements the error interface.
func (e *Error) Error() string {
	// Build error message using the first available format,
	// depending on which fields are set:
	//
	//   1. "<msg>: <err>" // base and wrapped error both set
	//   2. "<msg>"        // wrapped error is nil
	//   3. "<err>"        // base error message is empty
	//   4. ""             // no fields are set
	part := make([]string, 0, 2)

	if e.msg != "" {
		part = append(part, e.msg)
	}

	if e.err != nil {
		part = append(part, e.err.Error())
	}

	return strings.Join(part, ": ")
}

// Unwrap implements error unwrapping for errors.Is/As.
func (e *Error) Unwrap() error { return e.err }

// Is implements error matching for errors.Is, matching by message.
// This allows sentinel errors to match wrapped versions.
func (e *Error) Is(target error) bool {
	if te, ok := target.(*Error); ok {
		return e.msg == te.msg
	}

	return false
}

// LogValue implements slog.LogValuer for rich structured logging.
func (e *Error) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, len(e.attrs)+2)

	if e.msg != "" {
		attrs = append(attrs, slog.String("error", e.msg))
	}

	if e.err != nil {
		attrs = append(attrs, slog.String("cause", e.err.Error()))
	}

	return slog.GroupValue(append(attrs, e.attrs...)...)
}

// Wrap creates a new Error wrapping another error.
func (e *Error) Wrap(err error) *Error {
	return &Error{
		msg:   e.msg,
		err:   err,
		attrs: e.attrs, // Share attrs
	}
}

// With adds attributes to the error for structured logging.
// This creates a new Error instance to maintain immutability.
func (e *Error) With(attrs ...slog.Attr) *Error {
	newAttrs := make([]slog.Attr, len(e.attrs)+len(attrs))
	copy(newAttrs, e.attrs)
	copy(newAttrs[len(e.attrs):], attrs)

	return &Error{
		msg:   e.msg,
		err:   e.err,
		attrs: newAttrs,
	}
}

// WithPosition adds a position attribute to the error for better error
// messages.
func (e *Error) WithPosition(pos Position) *Error {
	return e.With(
		slog.Int("line", pos.Line),
		slog.Int("column", pos.Column),
		slog.Int("offset", pos.Offset),
	)
}
