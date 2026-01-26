package cli

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/log"
)

// logFormat is a custom type that configures the logger format as a side
// effect of parsing via encoding.TextUnmarshaler.
type logFormat string

// UnmarshalText implements encoding.TextUnmarshaler.
// As Kong parses the --log-format flag, this method is called, allowing us
// to configure the logger early enough to affect error messages during parsing.
func (f *logFormat) UnmarshalText(text []byte) error {
	*f = logFormat(text)
	log.Config(log.WithFormat(log.ParseFormat(string(*f))))

	return nil
}

// logLevel is a custom type that configures the logger level as a side
// effect of parsing via encoding.TextUnmarshaler.
type logLevel string

// UnmarshalText implements encoding.TextUnmarshaler.
// As Kong parses the --log-level flag, this method is called, allowing us
// to configure the logger early enough to affect error messages during parsing.
func (l *logLevel) UnmarshalText(text []byte) error {
	*l = logLevel(text)
	log.Config(log.WithLevel(log.ParseLevel(string(*l))))

	return nil
}

type logConfig struct {
	Level      logLevel  `default:"info"    enum:"debug,info,warn,error" help:"Set log level."`
	Format     logFormat `default:"json"    enum:"json,text"             help:"Set log format."`
	TimeLayout string    `default:"RFC3339"                              help:"Set timestamp format."`
	Caller     bool      `default:"false"                                help:"Include caller information."       negatable:""`
	Pretty     bool      `default:"true"                                 help:"Enable colorized pretty printing." negatable:""`
}

func (*logConfig) vars() kong.Vars {
	return kong.Vars{}
}

func (*logConfig) group() kong.Group {
	var group kong.Group

	group.Key = "log"
	group.Title = "Logging options"

	return group
}

func (f *logConfig) start(ctx context.Context) {
	log.Config(
		log.WithLevel(log.ParseLevel(string(f.Level))),
		log.WithFormat(log.ParseFormat(string(f.Format))),
		log.WithTimeLayout(f.TimeLayout),
		log.WithCaller(f.Caller),
		log.WithPretty(f.Pretty),
	)

	log.DebugContext(ctx, "logger initialized",
		slog.String("level", string(f.Level)),
		slog.String("format", string(f.Format)),
		slog.String("time", f.TimeLayout),
		slog.Bool("caller", f.Caller),
		slog.Bool("pretty", f.Pretty),
	)
}

// scan performs an early pass over command-line arguments to extract and
// apply logger configuration before Kong begins parsing. This ensures the
// logger is configured properly regardless of flag position on the command
// line.
//
// While logFormat and logLevel types implement encoding.TextUnmarshaler to
// configure the logger as flags are encountered during parsing, boolean flags
// like Pretty don't go through that interface. This pre-scan ensures all logger
// flags are applied early.
func (f *logConfig) scan(args []string) {
	type prefix struct {
		string

		len int
	}

	logPrefix := prefix{"--log-", 6}
	noLogPrefix := prefix{"--no-log-", 9}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Check if this is a log-related flag
		hasLogPrefix := len(arg) >= logPrefix.len &&
			arg[:logPrefix.len] == logPrefix.string

		hasNoLogPrefix := len(arg) >= noLogPrefix.len &&
			arg[:noLogPrefix.len] == noLogPrefix.string
		if !hasLogPrefix && !hasNoLogPrefix {
			continue
		}

		// Extract flag name and value
		var (
			name, value string
			assigned    bool
		)

		// Determine which prefix to use for parsing
		prefixLen := logPrefix.len
		if hasNoLogPrefix {
			prefixLen = noLogPrefix.len
		}

		if eq := len(arg); eq > prefixLen {
			for j := prefixLen; j < eq; j++ {
				if arg[j] == '=' {
					name, value = arg[:j], arg[j+1:]
					assigned = true

					break
				}
			}

			if name == "" {
				name = arg
			}
		}

		// Apply configuration
		switch name {
		case "--log-level":
			// Non-boolean flag: consume next arg as value if not assigned
			if !assigned && i+1 < len(args) && len(args[i+1]) > 0 &&
				args[i+1][0] != '-' {
				value = args[i+1]
				i++
			}

			_ = f.Level.UnmarshalText([]byte(value))

		case "--log-format":
			// Non-boolean flag: consume next arg as value if not assigned
			if !assigned && i+1 < len(args) && len(args[i+1]) > 0 &&
				args[i+1][0] != '-' {
				value = args[i+1]
				i++
			}

			_ = f.Format.UnmarshalText([]byte(value))

		case "--log-pretty":
			// Boolean flag: only parse value if explicitly assigned with =
			if assigned {
				v, err := strconv.ParseBool(value)
				if err == nil {
					f.Pretty = v
					log.Config(log.WithPretty(v))
				}
			} else {
				f.Pretty = true

				log.Config(log.WithPretty(true))
			}

		case "--no-log-pretty":
			// Boolean flag: only parse value if explicitly assigned with =
			if assigned {
				v, err := strconv.ParseBool(value)
				if err == nil {
					f.Pretty = !v
					log.Config(log.WithPretty(!v))
				}
			} else {
				f.Pretty = false

				log.Config(log.WithPretty(false))
			}

		case "--log-caller":
			// Boolean flag: only parse value if explicitly assigned with =
			if assigned {
				v, err := strconv.ParseBool(value)
				if err == nil {
					f.Caller = v
					log.Config(log.WithCaller(v))
				}
			} else {
				f.Caller = true

				log.Config(log.WithCaller(true))
			}

		case "--no-log-caller":
			// Boolean flag: only parse value if explicitly assigned with =
			if assigned {
				v, err := strconv.ParseBool(value)
				if err == nil {
					f.Caller = !v
					log.Config(log.WithCaller(!v))
				}
			} else {
				f.Caller = false

				log.Config(log.WithCaller(false))
			}
		}
	}
}
