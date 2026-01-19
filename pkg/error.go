package pkg

// Sentinel errors for the envcomp package and its subpackages.
// These errors can be tested using errors.Is for reliable error checking.

import (
	"fmt"
	"slices"
	"strings"
)

// Error represents a chain of errors.
type Error []error

// ErrNoParseTree is returned when the parser generates no parse tree.
//
// This typically indicates an empty or invalid input that was consumed
// by the parser without producing a valid abstract syntax tree.
var ErrNoParseTree = MakeErrorf("no parse tree generated")

// ErrReadStdin is returned when reading from standard input fails.
//
// This error should be wrapped with the underlying I/O error
// to preserve the error chain.
var ErrReadStdin = MakeErrorf("failed to read stdin")

// ErrParse is returned when parsing input fails.
//
// This error should be wrapped with the underlying parse error
// to preserve the error chain and detailed parse error information.
var ErrParse = MakeErrorf("parse error")

// ErrUnknownConfigKey is returned when setting an unknown configuration key.
//
// This error indicates that a requested configuration key does not exist
// in the provided configuration source.
// var ErrUnknownConfigKey = MakeErrorf("unknown configuration key")

// ErrJSONMarshal is returned when JSON marshaling fails.
//
// This error should be wrapped with the underlying marshaling error
// to preserve the error chain.
var ErrJSONMarshal = MakeErrorf("JSON marshal error")

// ErrYAMLMarshal is returned when YAML marshaling fails.
//
// This error should be wrapped with the underlying marshaling error
// to preserve the error chain.
var ErrYAMLMarshal = MakeErrorf("YAML marshal error")

// ErrInvalidFormat is returned when an invalid format is specified.
//
// This error should be wrapped with additional context that specifies the
// invalid format along with a list of valid formats.
var ErrInvalidFormat = MakeErrorf("invalid format")

// ErrAmbiguousParse is returned when the parser detects an ambiguous parse.
//
// This error indicates that the input can be parsed in multiple ways,
// which is not allowed. The error should be wrapped with details about
// the ambiguity location.
var ErrAmbiguousParse = MakeErrorf("the input can be parsed in multiple ways")

// ErrInvalidAlternate is returned when an invalid grammar alternate is
// encountered.
//
// This error indicates an internal parser error where an unexpected
// alternate index was encountered during AST construction.
var ErrInvalidAlternate = MakeErrorf("invalid grammar alternate")

// ErrMaxDepthExceeded is returned when maximum definition depth is exceeded.
//
// This error indicates possible infinite recursion in the definition chain.
// The error should be wrapped with the recursion chain context.
var ErrMaxDepthExceeded = MakeErrorf("maximum definition depth exceeded")

// ErrDefinitionNotFound is returned when a requested definition is not found.
//
// This error should be wrapped with the name of the definition that was
// not found.
var ErrDefinitionNotFound = MakeErrorf("definition not found")

// ErrReadInput is returned when reading input fails.
//
// This error should be wrapped with the underlying I/O error
// to preserve the error chain.
var ErrReadInput = MakeErrorf("failed to read input")

// MakeError constructs an Error from the given errors.
// The errors are stored in the order they are provided:
// the first argument is the innermost error in the chain.
// Nil is returned if no errors are provided.
func MakeError(errs ...error) Error {
	var e Error

	for _, err := range errs {
		if err != nil {
			e = append(e, UnwrapErrors(err)...)
		}
	}

	return e
}

// MakeErrorf constructs an Error from a formatted error message.
func MakeErrorf(format string, args ...any) Error {
	return MakeError(fmt.Errorf(format, args...))
}

// Error returns a concatenated string representation of all errors
// in the error chain, separated by ": ", from innermost to outermost.
func (e Error) Error() string {
	var sb strings.Builder

	for i, err := range slices.All(e) {
		if i > 0 {
			sb.WriteString(": ")
		}

		sb.WriteString(err.Error())
	}

	return sb.String()
}

// Wrap appends one or more errors to the receiver and returns the result.
func (e Error) Wrap(err ...error) Error {
	return append(e, err...)
}

// Wrapf appends a formatted error to the receiver and returns the result.
func (e Error) Wrapf(format string, args ...any) Error {
	return append(e, fmt.Errorf(format, args...))
}

// Unwrap returns the slice of errors contained in the receiver.
func (e Error) Unwrap() []error {
	return e
}

// UnwrapErrors recursively unwraps an error chain and returns a slice
// containing all errors in the chain, starting from the innermost error.
func UnwrapErrors(err error) Error {
	if err == nil {
		return nil
	}

	chain := Error{}

	if e, ok := err.(interface{ Unwrap() []error }); ok {
		for _, wrapped := range e.Unwrap() {
			chain = append(chain, UnwrapErrors(wrapped)...)
		}
	} else if e, ok := err.(interface{ Unwrap() error }); ok {
		chain = append(chain, UnwrapErrors(e.Unwrap())...)
	}

	return append(chain, err)
}
