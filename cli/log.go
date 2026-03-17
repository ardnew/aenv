package cli

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

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

var DefaultLogOutput = os.Stderr

type logConfig struct {
	Level      logLevel  `default:"info"    enum:"${logLevelEnum}"  help:"Set log level (${enum})"          short:"g"`
	Format     logFormat `default:"text"    enum:"${logFormatEnum}" help:"Set log format (${enum})"         short:"a"`
	Output     string    `default:"-"                               help:"Log output file ('-' for stderr)" short:"o" placeholder:"PATH" type:"path"`
	TimeLayout string    `default:"RFC3339"                         help:"Set timestamp format"`
	Callsite   bool      `default:"false"                           help:"Include callsite information"     short:"c"                                negatable:""`
	Pretty     bool      `default:"true"                            help:"Enable colorized pretty printing" short:"p"                                negatable:""`
}

func (*logConfig) vars() kong.Vars {
	return kong.Vars{
		"logLevelEnum":  strings.Join(slices.Collect(log.Levels()), ","),
		"logFormatEnum": strings.Join(slices.Collect(log.Formats()), ","),
	}
}

func (*logConfig) group() kong.Group {
	var group kong.Group

	group.Key = "log"
	group.Title = "Logging options"

	return group
}

func (f *logConfig) start(ctx context.Context, verbose int, quiet bool) func() {
	// Apply verbosity to log level
	level := f.applyVerbosity(verbose, quiet)

	// Open output destination
	path := strings.TrimLeft(f.Output, " \t")

	var (
		outputWriter io.Writer
		outputStr    string
		closeOutput  func() error
	)

	if strings.TrimSpace(path) == "-" {
		outputWriter = DefaultLogOutput
		outputStr = "-"
		closeOutput = func() error { return nil }
	} else {
		outputWriter, outputStr, closeOutput = openLogFile(ctx, path)
	}

	log.Config(
		log.WithLevel(level),
		log.WithFormat(log.ParseFormat(string(f.Format))),
		log.WithTimeLayout(f.TimeLayout),
		log.WithCallsite(f.Callsite),
		log.WithPretty(f.Pretty),
		log.WithOutput(outputWriter),
	)

	logAttrs := []slog.Attr{
		slog.String("level", level.String()),
		slog.String("format", string(f.Format)),
		slog.String("time", f.TimeLayout),
		slog.Bool("callsite", f.Callsite),
		slog.Bool("pretty", f.Pretty),
		slog.String("output", outputStr),
	}
	if verbose > 0 {
		logAttrs = append(logAttrs, slog.Int("verbose", verbose))
	}

	log.TraceContext(ctx, "logger initialized", logAttrs...)

	return func() {
		err := closeOutput()
		if err != nil {
			log.ErrorContext(ctx, "close log output file",
				slog.String("error", err.Error()),
			)
		}
	}
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
	// Create flag set for log configuration parsing
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Suppress error output during pre-scan

	// Define log-related flags
	var (
		level      = fs.String("log-level", "", "")
		format     = fs.String("log-format", "", "")
		pretty     = fs.Bool("log-pretty", false, "")
		noPretty   = fs.Bool("no-log-pretty", false, "")
		callsite   = fs.Bool("log-callsite", false, "")
		noCallsite = fs.Bool("no-log-callsite", false, "")
		verbose    = fs.Int("verbose", 0, "")
		quiet      = fs.Bool("quiet", false, "")
	)
	fs.IntVar(verbose, "v", 0, "")    // Short form alias
	fs.BoolVar(quiet, "q", false, "") // Short form alias

	var output string
	fs.StringVar(&output, "log-output", "", "")
	fs.StringVar(&output, "o", "", "") // Short form alias

	// Extract only log-related flags and their values from args
	var logArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Check for long-form log flags or short flags
		isLongLog := strings.HasPrefix(arg, "--log-") ||
			strings.HasPrefix(arg, "--no-log-")
		isShortOutput := arg == "-o" || strings.HasPrefix(arg, "-o=")
		isQuiet := arg == "-q" || arg == "--quiet"

		// Check for -v flags (can be stacked like -vv or -vvv)
		hasVerbose := false

		if len(arg) > 1 && arg[0] == '-' && arg[1] == 'v' {
			// Check if it's all v's (like -v, -vv, -vvv)
			allV := true

			for j := 1; j < len(arg); j++ {
				if arg[j] != 'v' {
					allV = false

					break
				}
			}

			if allV {
				// Stacked verbose flags
				verbose = new(len(arg) - 1)
				hasVerbose = true
			} else if strings.HasPrefix(arg, "-v=") {
				hasVerbose = true
			}
		}

		if isLongLog || isShortOutput || hasVerbose || isQuiet {
			if !hasVerbose || strings.Contains(arg, "=") {
				// Add to logArgs for flag parsing (skip already-handled stacked -vv)
				logArgs = append(logArgs, arg)
			}
			// Include next arg if it's a value (no '=' in current arg, next isn't a
			// flag)
			if !strings.Contains(arg, "=") &&
				i+1 < len(args) &&
				!strings.HasPrefix(args[i+1], "-") {
				i++
				logArgs = append(logArgs, args[i])
			}
		}
	}

	// Parse log arguments (ignore errors - Kong will validate later)
	_ = fs.Parse(logArgs)

	// Apply parsed string configuration
	if *level != "" {
		_ = f.Level.UnmarshalText([]byte(*level))
	}

	// Apply verbosity and configure log level
	lvl := f.applyVerbosity(*verbose, *quiet)
	log.Config(log.WithLevel(lvl))

	if *format != "" {
		_ = f.Format.UnmarshalText([]byte(*format))
	}

	if output != "" {
		f.Output = output
	}

	// Apply boolean flags if explicitly set
	if *pretty {
		f.Pretty = true

		log.Config(log.WithPretty(true))
	}

	if *noPretty {
		f.Pretty = false

		log.Config(log.WithPretty(false))
	}

	if *callsite {
		f.Callsite = true

		log.Config(log.WithCallsite(true))
	}

	if *noCallsite {
		f.Callsite = false

		log.Config(log.WithCallsite(false))
	}
}

// levelStep is the numeric gap between adjacent named log levels.
// This mirrors the slog convention where levels are spaced 4 apart
// (e.g., trace=-8, debug=-4, info=0, warn=4, error=8).
const levelStep = 4

// applyVerbosity determines the effective log level by adjusting the
// configured level relatively based on the verbosity count. Each -v flag
// increases verbosity by one named level (decreases the numeric level by
// [levelStep]). The result is clamped to [log.LevelTrace] as the minimum.
//
// Examples (assuming default level is info):
//
//	--log-level=info  -v   => info  + 1 = debug
//	--log-level=error -vv  => error + 2 = info
//	-v --log-level=error   => error + 1 = warn
//	--log-level=warn  -v   => warn  + 1 = info
func (f *logConfig) applyVerbosity(verbose int, quiet bool) log.Level {
	if quiet {
		return log.LevelError
	}

	base := log.ParseLevel(string(f.Level))
	adjusted := base - log.Level(verbose*levelStep)

	if adjusted < log.LevelTrace {
		return log.LevelTrace
	}

	return adjusted
}

// openLogFile opens (or creates) the log output file at path. It handles the
// ">>" prefix for append mode, returning the writer, a display string for the
// path, and a close function.
func openLogFile(
	ctx context.Context,
	path string,
) (io.Writer, string, func() error) {
	flags := os.O_CREATE | os.O_WRONLY

	var ok bool
	if path, ok = strings.CutPrefix(path, ">>"); ok {
		path = strings.TrimLeft(path, " \t")
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		log.ErrorContext(ctx, "open log output file",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	outputStr := path
	if flags&os.O_APPEND != 0 {
		outputStr += " (APPEND)"
	}

	return file, outputStr, file.Close
}
