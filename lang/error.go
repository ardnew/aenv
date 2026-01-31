package lang

import (
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/ardnew/aenv/lang/parser"
)

// Predefined errors (sentinel values).
var (
	ErrNoParseTree        = NewError("no parse tree generated")
	ErrInvalidToken       = NewError("invalid token")
	ErrAmbiguousParse     = NewError("ambiguous parse")
	ErrMaxDepthExceeded   = NewError("maximum definition depth exceeded")
	ErrDefinitionNotFound = NewError("definition not found")
	ErrReadInput          = NewError("failed to read input")
	ErrExprCompile        = NewError("expression compilation failed")
	ErrExprEvaluate       = NewError("expression evaluation failed")
	ErrParamCountMismatch = NewError("parameter count mismatch")
	ErrInvalidValueType   = NewError("invalid value type")
	ErrInvalidBoolean     = NewError("invalid boolean value")
	ErrInvalidNumber      = NewError("invalid number value")
)

// Error represents an error with optional structured logging attributes.
// It implements both error and slog.LogValuer interfaces.
type Error struct {
	msg   string
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

// ParseError wraps parser errors.
type ParseError struct {
	Errors   []*parser.Error
	Source   string   // The original source input
	Snippet  string   // Optional snippet of the source
	Expected []string // Optional expected tokens
}

func NewParseError(errors []*parser.Error, source string) *ParseError {
	return &ParseError{
		Errors:   errors,
		Source:   source,
		Snippet:  "",
		Expected: nil,
	}
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if len(e.Errors) == 0 {
		return "parse error"
	}

	// If we have the source, format with context
	if e.Source != "" {
		msg, snippet, expected := e.formatWithContext()
		e.Snippet = snippet
		e.Expected = expected

		return msg + snippet + "\texpected: " + strings.Join(expected, ", ")
	}

	return e.Errors[0].String()
}

// formatWithContext formats the parse error with source code context.
func (e *ParseError) formatWithContext() (string, string, []string) {
	if len(e.Errors) == 0 {
		return "parse error", "", nil
	}

	firstErr := e.Errors[0]
	lines := strings.Split(e.Source, "\n")

	var buf, src strings.Builder

	// Write error location and description
	buf.WriteString("parse error at line ")
	buf.WriteString(strconv.Itoa(firstErr.Line))
	buf.WriteString(", column ")
	buf.WriteString(strconv.Itoa(firstErr.Column))
	buf.WriteString(":\n")

	// Show the offending line if within bounds
	if firstErr.Line > 0 && firstErr.Line <= len(lines) {
		lineIdx := firstErr.Line - 1
		line := lines[lineIdx]

		// Print the line with line number
		src.WriteString("  ")
		src.WriteString(strconv.Itoa(firstErr.Line))
		src.WriteString(" | ")
		src.WriteString(line)
		src.WriteRune('\n')

		// Print marker pointing to the column
		// Calculate the width needed for line number display
		lineNumWidth := len(strconv.Itoa(firstErr.Line))
		// +5 accounts for: 2 leading spaces + " | " (3 chars)
		padding := strings.Repeat(" ", lineNumWidth+5)

		// Add spaces to reach the error column
		if firstErr.Column > 0 {
			padding += strings.Repeat(" ", firstErr.Column-1)
		}

		src.WriteString(padding + "^\n")
	}

	// Write what was expected
	exp := []string{}
	for _, e := range firstErr.Expected {
		exp = append(exp, strconv.Quote(e))
	}

	slices.Sort(exp)

	return buf.String(), src.String(), exp
}
