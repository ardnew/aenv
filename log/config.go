package log

import (
	"io"

	"golang.org/x/term"
)

// Level is a log severity. Higher values are more verbose. The zero value is
// invalid.
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
		return errf(ErrInvalidLevel, "%s", text)
	}
	return nil
}

// Symbol returns the level's terminal badge.
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

// LevelRange returns the lowest and highest valid levels.
func LevelRange() (min, max Level) { return levelMin, levelMax }

// Valid reports whether the level is recognized.
func (l Level) Valid() bool {
	return l >= levelMin && l <= levelMax
}

// Allows reports whether a handler at this level forwards a record at the given
// level.
func (l Level) Allows(record Level) bool {
	return l.Valid() && record.Valid() && record <= l
}

// Format is a Handler output encoding. The zero value is invalid.
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
		return errf(ErrInvalidFormat, "%s", text)
	}
	return nil
}

// FormatRange returns the lowest and highest valid formats.
func FormatRange() (min, max Format) { return formatMin, formatMax }

// Valid reports whether the format is recognized.
func (f Format) Valid() bool {
	return f >= formatMin && f <= formatMax
}

// HandlerOptions configures a Handler.
type HandlerOptions struct {
	// Writer is the output. Required.
	Writer io.Writer
	// Format is the output encoding.
	Format Format
	// Level is the highest level forwarded; higher levels are dropped.
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
		return handlerConfig{}, ErrNilWriter
	}
	if !options.Format.Valid() {
		return handlerConfig{}, errf(ErrInvalidFormat, "%d", options.Format)
	}
	if !options.Level.Valid() {
		return handlerConfig{}, errf(ErrInvalidLevel, "%d", options.Level)
	}
	return handlerConfig{
		writer:  options.Writer,
		format:  options.Format,
		level:   options.Level,
		enabled: true,
		target:  detectOutputTarget(options.Writer),
	}, nil
}

// TerminalWriter lets a writer declare itself a terminal, so callers can mark
// in-memory writers as terminals for format and source defaults.
type TerminalWriter interface {
	// IsTerminalWriter reports whether the writer targets a terminal.
	IsTerminalWriter() bool
}

// IsTerminal reports whether writer targets a terminal. A writer qualifies if
// it implements TerminalWriter and reports true, or exposes an Fd that
// term.IsTerminal recognizes.
func IsTerminal(writer io.Writer) bool {
	marker, ok := writer.(TerminalWriter)
	if ok && marker.IsTerminalWriter() {
		return true
	}
	descriptor, ok := writer.(interface{ Fd() uintptr })
	if ok && term.IsTerminal(int(descriptor.Fd())) {
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
