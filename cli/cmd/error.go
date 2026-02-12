package cmd

import (
	"log/slog"
	"strings"
)

// Error represents a CLI command error with structured logging support.
type Error struct {
	msg   string
	err   error
	attrs []slog.Attr
}

func NewError(msg string) *Error {
	return &Error{msg: msg}
}

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

func (e *Error) Unwrap() error { return e.err }

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

var (
	ErrJSONMarshal = NewError("marshal JSON")
	ErrYAMLMarshal = NewError("marshal YAML")
	ErrWriteConfig = NewError("write configuration file")
	ErrFileExists  = NewError("file exists (use --force to overwrite)")
)
