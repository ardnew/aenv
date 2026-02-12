package log

//go:generate go tool stringer --linecomment --type Level,Format --output config_string.go

import (
	"io"
	"iter"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Level represents the severity of a log message.
type Level slog.Level

const levelTraceMask = -8

const (
	LevelTrace Level = Level(levelTraceMask)  // trace
	LevelDebug Level = Level(slog.LevelDebug) // debug
	LevelInfo  Level = Level(slog.LevelInfo)  // info
	LevelWarn  Level = Level(slog.LevelWarn)  // warn
	LevelError Level = Level(slog.LevelError) // error
)

// DefaultLevel is the default log level.
const DefaultLevel = LevelInfo

// Levels returns an iterator over all defined log levels.
func Levels() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, level := range []Level{
			LevelTrace,
			LevelDebug,
			LevelInfo,
			LevelWarn,
			LevelError,
		} {
			if !yield(level.String()) {
				return
			}
		}
	}
}

// ParseLevel parses a string representation of a log level.
// Valid level strings are "TRACE", "DEBUG", "INFO", "WARN", and "ERROR",
// optionally followed by a "+" or "-" and an integer offset.
// See [slog.Level.UnmarshalText] for details.
func ParseLevel(s string) Level {
	// Check for "trace" explicitly since slog.Level.UnmarshalText doesn't
	// recognize it
	if strings.EqualFold(s, "trace") {
		return LevelTrace
	}

	l := new(slog.Level)

	err := l.UnmarshalText([]byte(s))
	if err != nil {
		return DefaultLevel
	}

	return Level(*l)
}

// Format represents the output format for log messages.
type Format int

const (
	FormatText Format = iota // text
	FormatJSON               // json
)

// DefaultFormat is the default log message format.
const DefaultFormat = FormatJSON

// Formats returns an iterator over all defined log formats.
func Formats() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, format := range []Format{
			FormatJSON,
			FormatText,
		} {
			if !yield(format.String()) {
				return
			}
		}
	}
}

// ParseFormat parses a string representation of a log format.
// Valid format strings are "json" and "text".
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		return DefaultFormat
	}
}

// FormatTime defines a function that formats a time.Time value as a string.
type FormatTime func(time.Time) string

// DefaultTimeLayout is the default used when no valid time layout is provided.
const DefaultTimeLayout = time.RFC3339

// DefaultCaller is the default setting for including caller information
// in log output.
const DefaultCaller = false

// DefaultPretty is the default setting for pretty printing log output.
const DefaultPretty = true

// config holds the configuration options for a Logger.
type config struct {
	mutex      *sync.RWMutex
	output     io.Writer
	formatTime FormatTime
	level      Level
	format     Format
	caller     bool
	pretty     bool
}

// makeConfig creates a new config with defaults applied, overridden by any
// provided options.
func makeConfig(w io.Writer, opts ...Option) config {
	var c config

	c.mutex = &sync.RWMutex{}

	return apply(apply(c, WithDefaults(w)), opts...)
}

// clone creates a copy of the config with a separate mutex and applies any
// provided options.
func (c config) clone(opts ...Option) config {
	c.mutex = &sync.RWMutex{}

	return apply(c, opts...)
}

// handler creates a slog.Handler based on the current configuration.
// The optional opts can be used to override specific configuration values.
func (c config) handler(opts ...Option) slog.Handler {
	// makeOpts is not called unless needed.
	makeOpts := func(cfg config) (io.Writer, *slog.HandlerOptions) {
		return cfg.output, &slog.HandlerOptions{
			AddSource: cfg.caller,
			Level:     slog.Level(cfg.level),
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					if t, ok := a.Value.Any().(time.Time); ok {
						formatted := cfg.formatTime(t)
						if formatted == "" {
							return slog.Attr{}
						}

						a.Value = slog.StringValue(formatted)
					}
				}

				// Replace level with custom string representation to show
				// "TRACE" instead of "DEBUG-4". Use uppercase to match slog's
				// default level formatting.
				if a.Key == slog.LevelKey {
					if level, ok := a.Value.Any().(slog.Level); ok {
						a.Value = slog.StringValue(strings.ToUpper(Level(level).String()))
					}
				}

				return a
			},
		}
	}

	override := apply(c, opts...)

	// Use pretty handlers if enabled
	if override.pretty {
		out, opt := makeOpts(override)

		switch override.format {
		case FormatJSON:
			return newPrettyJSONHandler(out, opt)

		case FormatText:
			return newPrettyTextHandler(out, opt)

		default:
			return slog.DiscardHandler
		}
	}

	switch override.format {
	case FormatJSON:
		out, opt := makeOpts(override)

		return slog.NewJSONHandler(out, opt)

	case FormatText:
		out, opt := makeOpts(override)

		return slog.NewTextHandler(out, opt)

	default:
		return slog.DiscardHandler
	}
}

// WithDefaults returns a functional option that sets the default configuration.
// The default configuration is [DefaultTimeLayout], [DefaultLevel],
// [DefaultFormat], and caller info disabled.
func WithDefaults(w io.Writer) Option {
	return func(c config) config {
		if w == nil {
			w = io.Discard
		}

		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.output = w
		c.formatTime = makeFormatTimeFunc(DefaultTimeLayout)
		c.level = DefaultLevel
		c.format = DefaultFormat
		c.caller = DefaultCaller
		c.pretty = DefaultPretty

		return c
	}
}

// WithOutput returns a functional option that sets the output [io.Writer]
// for log messages.
// If a nil writer is provided, [io.Discard] is used instead.
func WithOutput(w io.Writer) Option {
	return func(c config) config {
		if w == nil {
			w = io.Discard
		}

		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.output = w

		return c
	}
}

// WithLevel returns a functional option that sets the minimum log level.
// Messages below this level are discarded.
func WithLevel(level Level) Option {
	return func(c config) config {
		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.level = level

		return c
	}
}

// WithFormat returns a functional option that sets the output format
// for log messages.
func WithFormat(format Format) Option {
	return func(c config) config {
		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.format = format

		return c
	}
}

// WithTimeLayout returns a functional option that sets the layout used to
// format log timestamps.
//
// The layout string can be one of the named layouts from the [time] package
// (for example, "RFC3339" or "RFC3339Nano"). Otherwise, it is passed verbatim
// to [time.Time.Format] and must follow the standard specification.
//
// If an empty string (after trimming whitespace) is provided, timestamps are
// disabled and no time is included in log output.
func WithTimeLayout(layout string) Option {
	return func(c config) config {
		format := makeFormatTimeFunc(layout)

		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.formatTime = format

		return c
	}
}

// WithCaller returns a functional option that controls whether caller
// information is included in log output.
func WithCaller(enable bool) Option {
	return func(c config) config {
		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.caller = enable

		return c
	}
}

// WithPretty returns a functional option that controls whether log output
// uses pretty printing with colors and formatting.
// For text format: removes quotes, uses colors for keys (gray) and values.
// For JSON format: multiline with indentation and colors.
func WithPretty(enable bool) Option {
	return func(c config) config {
		if c.mutex == nil {
			c.mutex = &sync.RWMutex{}
		} else {
			c.mutex.Lock()
			defer c.mutex.Unlock()
		}

		c.pretty = enable

		return c
	}
}

// timeLayout maps named layouts to their corresponding time.Time constants.
var timeLayout = map[string]string{
	"rfc3339":     time.RFC3339,
	"rfc3339nano": time.RFC3339Nano,
	"ansic":       time.ANSIC,
	"unixdate":    time.UnixDate,
	"rubydate":    time.RubyDate,
	"rfc822":      time.RFC822,
	"rfc822z":     time.RFC822Z,
	"rfc850":      time.RFC850,
	"kitchen":     time.Kitchen,

	"stamp": time.Stamp,
	"none":  "",

	"stampmilli": time.StampMilli,
	"milli":      time.StampMilli,
	"millis":     time.StampMilli,
	"ms":         time.StampMilli,

	"stampmicro": time.StampMicro,
	"micro":      time.StampMicro,
	"micros":     time.StampMicro,
	"us":         time.StampMicro,

	"stampnano": time.StampNano,
	"nano":      time.StampNano,
	"nanos":     time.StampNano,
	"ns":        time.StampNano,
}

func makeFormatTimeFunc(layout string) FormatTime {
	// Trim whitespace only for inspection.
	// Custom layouts are used verbatim.
	trimmed := strings.Map(
		func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}

			return -1
		},
		strings.ToLower(layout),
	)

	if trimmed == "" {
		return func(time.Time) string { return "" }
	}

	if std, ok := timeLayout[trimmed]; ok {
		layout = std
	}

	return func(t time.Time) string { return t.Format(layout) }
}
