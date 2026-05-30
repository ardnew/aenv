package log

import (
	"fmt"
	"io"

	"golang.org/x/term"
)

// Level is log severity ordered by increasing verbosity.
// The zero value is invalid; variables must be explicitly initialized.
type Level uint8

//go:generate go tool stringer -linecomment -type=Level
const (
	LevelError Level = iota + 1 // error
	LevelWarn                   // warn
	LevelInfo                   // info
	LevelDebug                  // debug
	LevelTrace                  // trace

	levelMin = LevelError
	levelMax = LevelTrace
)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (l *Level) UnmarshalText(text []byte) error {
	switch string(text) {
	case "error":
		*l = LevelError
	case "warn":
		*l = LevelWarn
	case "info":
		*l = LevelInfo
	case "debug":
		*l = LevelDebug
	case "trace":
		*l = LevelTrace
	default:
		return fmt.Errorf("log: invalid level: %s", text)
	}
	return nil
}

// Symbol returns the terminal badge for the level (for example, "+" or ":").
func (l Level) Symbol() string {
	switch l {
	case LevelError:
		return "="
	case LevelWarn:
		return "-"
	case LevelInfo:
		return " "
	case LevelDebug:
		return "·"
	case LevelTrace:
		return ":"
	default:
		return "?"
	}
}

// LevelRange returns the inclusive range of valid log levels.
func LevelRange() (min, max Level) { return levelMin, levelMax }

// Valid reports whether the level is a recognized severity.
func (l Level) Valid() bool {
	return l >= levelMin && l <= levelMax
}

// Allows reports whether a handler configured at this level would forward a
// record at the given level.
func (l Level) Allows(record Level) bool {
	return l.Valid() && record.Valid() && record <= l
}

// Format is the output encoding for a Handler.
// The zero value is invalid; variables must be explicitly initialized.
type Format uint8

//go:generate go tool stringer -linecomment -type=Format
const (
	FormatText Format = iota + 1 // text
	FormatJSON                   // json

	formatMin = FormatText
	formatMax = FormatJSON
)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (f *Format) UnmarshalText(text []byte) error {
	switch string(text) {
	case "text":
		*f = FormatText
	case "json":
		*f = FormatJSON
	default:
		return fmt.Errorf("log: invalid format: %s", text)
	}
	return nil
}

// FormatRange returns the inclusive range of valid log formats.
func FormatRange() (min, max Format) { return formatMin, formatMax }

// Valid reports whether the format is a recognized log format.
func (f Format) Valid() bool {
	return f >= formatMin && f <= formatMax
}

// HandlerOptions configures a new Handler.
type HandlerOptions struct {
	// Writer is the output destination; must not be nil.
	Writer io.Writer
	// Format is the output encoding: FormatText or FormatJSON.
	Format Format
	// Level is the maximum level forwarded by this handler;
	// events above this are silenced.
	Level Level
}

type (
	handlerConfig struct {
		writer  io.Writer
		format  Format
		level   Level
		enabled bool
		target  outputTarget
	}
	handlerCandidate struct {
		handler *Handler
		config  handlerConfig
	}
)

type outputTarget uint8

const (
	targetFile outputTarget = iota
	targetTerminal
)

func newHandlerConfig(options HandlerOptions) (handlerConfig, error) {
	if options.Writer == nil {
		return handlerConfig{}, fmt.Errorf("log: nil writer")
	}
	if !options.Format.Valid() {
		return handlerConfig{}, fmt.Errorf("log: invalid format %d", options.Format)
	}
	if !options.Level.Valid() {
		return handlerConfig{}, fmt.Errorf("log: invalid level %d", options.Level)
	}
	return handlerConfig{
		writer:  options.Writer,
		format:  options.Format,
		level:   options.Level,
		enabled: true,
		target:  detectOutputTarget(options.Writer),
	}, nil
}

// TerminalWriter is implemented by writers that target a terminal regardless of
// any underlying file descriptor. It lets callers mark in-memory writers (for
// example, test buffers) as terminals so format and source defaults apply.
type TerminalWriter interface {
	// IsTerminalWriter reports whether the writer targets a terminal.
	IsTerminalWriter() bool
}

// IsTerminal reports whether writer is connected to a terminal.
// A writer is treated as a terminal if it implements TerminalWriter and reports
// true, or if it exposes an Fd that term.IsTerminal recognizes.
func IsTerminal(writer io.Writer) bool {
	if marker, ok := writer.(TerminalWriter); ok && marker.IsTerminalWriter() {
		return true
	}
	if descriptor, ok := writer.(interface{ Fd() uintptr }); ok && term.IsTerminal(int(descriptor.Fd())) {
		return true
	}
	return false
}

func detectOutputTarget(writer io.Writer) outputTarget {
	if IsTerminal(writer) {
		return targetTerminal
	}
	return targetFile
}
