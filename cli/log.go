package cli

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/log"
)

const logHandlerSyntax = "output[,format[,level]]"

// logHandlerSpec represents a log handler specified via command-line flags.
//
// Log handlers are used to filter, format, and route log messages.
// Each message is routed to all handlers of sufficient severity level, and
// each handler formats the message for its output.
//
// If no handlers are specified, the default handler is "stdout,text,warn".
//
// All fields of a handler specification are optional, and delimiters are only
// required if a trailing field is specified. The default format and level are
// output-specific.
//
// The default log level is "info" for user-added handlers, "warn" by default.
// The default log format is "text" for terminals, "json" otherwise.
//
// Handlers specified with the same output will be merged by overriding previous
// fields with later non-empty fields, and modifying the default handler automatically
// promotes its level to info.
//
// For example, ",json" updates the default handler to "stdout,json,info".
type logHandlerSpec struct {
	output    string     // "stdout" (or "-"), "stderr", or a file path
	format    log.Format // "text" or "json"
	level     log.Level  // "error", "warn", "info", "debug", or "trace"
	formatSet bool
	levelSet  bool
}

func (s *logHandlerSpec) UnmarshalText(text []byte) error {
	fields := strings.Split(string(text), ",")
	if len(fields) > 3 {
		return Error{
			Err:  errf(log.ErrInvalidHandler, "expected %s", logHandlerSyntax),
			Code: exit.Usage,
		}
	}
	parsed := logHandlerSpec{level: log.LevelInfo}
	if len(fields) > 0 {
		parsed.output = fields[0]
	}
	if len(fields) > 1 && fields[1] != "" {
		format, err := parseLogFormat(fields[1])
		if err != nil {
			return Error{Err: err, Code: exit.Usage}
		}
		parsed.format = format
		parsed.formatSet = true
	}
	if len(fields) > 2 && fields[2] != "" {
		level, err := parseLogLevel(fields[2])
		if err != nil {
			return Error{Err: err, Code: exit.Usage}
		}
		parsed.level = level
		parsed.levelSet = true
	}

	*s = parsed
	return nil
}

func parseLogFormat(value string) (log.Format, error) {
	var format log.Format
	err := format.UnmarshalText([]byte(value))
	return format, err
}

func parseLogLevel(value string) (log.Level, error) {
	var level log.Level
	err := level.UnmarshalText([]byte(value))
	return level, err
}

func openLogHandler(specs []logHandlerSpec, verbose int) ([]io.Closer, error) {
	closers := []io.Closer{}
	options := []log.HandlerOptions{}
	specs = mergeLogHandlerSpecs(specs)
	configured := make([][]slog.Attr, 0, len(specs))
	replaceConsole := false

	for _, spec := range specs {
		writer, closer, console, err := resolveLogWriter(spec.output)
		if err != nil {
			_ = closeLogHandlers(closers)
			return nil, err
		}
		format := resolveFormat(spec, writer)
		level := spec.level
		if !level.Valid() {
			level = log.LevelInfo
		}
		level = adjustLevel(level, verbose)
		if closer != nil {
			closers = append(closers, closer)
		}
		if console {
			replaceConsole = true
		}
		configured = append(configured, log.Attrs(
			"output", spec.output,
			"format", format.String(),
			"level", level.String(),
			"console", console,
		))
		options = append(options, log.HandlerOptions{
			Writer: writer,
			Format: format,
			Level:  level,
		})
	}

	if !replaceConsole {
		configured = append(configured, log.Attrs(
			"output", "stdout",
			"format", log.FormatText.String(),
			"level", adjustLevel(log.LevelWarn, verbose).String(),
			"console", true,
		))
		options = append([]log.HandlerOptions{{
			Writer: os.Stdout,
			Format: log.FormatText,
			Level:  adjustLevel(log.LevelWarn, verbose),
		}}, options...)
	}

	driver, err := log.New(options...)
	if err != nil {
		_ = closeLogHandlers(closers)
		return nil, err
	}
	log.SetDefault(driver)
	log.Debug(log.Attrs(
		"handlers", len(options),
		"verbose", verbose,
		"default-console", !replaceConsole,
	), "logging configured")
	for _, attr := range configured {
		log.Trace(attr)
	}
	return closers, nil
}

func mergeLogHandlerSpecs(specs []logHandlerSpec) []logHandlerSpec {
	if len(specs) < 2 {
		return specs
	}
	order := make([]string, 0, len(specs))
	merged := make(map[string]logHandlerSpec, len(specs))
	for _, spec := range specs {
		key := logOutputKey(spec.output)
		previous, ok := merged[key]
		if !ok {
			order = append(order, key)
		} else {
			if !spec.formatSet {
				spec.format = previous.format
				spec.formatSet = previous.formatSet
			}
			if !spec.levelSet {
				spec.level = previous.level
				spec.levelSet = previous.levelSet
			}
		}
		merged[key] = spec
	}
	specs = specs[:0]
	for _, key := range order {
		specs = append(specs, merged[key])
	}
	return specs
}

func logOutputKey(output string) string {
	switch output {
	case "", "-", "stdout":
		return "stdout"
	case "stderr":
		return "stderr"
	default:
		return kong.ExpandPath(output)
	}
}

func resolveLogWriter(output string) (io.Writer, io.Closer, bool, error) {
	switch output {
	case "", "-", "stdout":
		return os.Stdout, nil, true, nil
	case "stderr":
		return os.Stderr, nil, true, nil
	}

	path := kong.ExpandPath(output)
	if file, ok := consoleFile(path); ok {
		return file, nil, true, nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, nil, false, err
	}

	return file, file, false, nil
}

func consoleFile(path string) (*os.File, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if stdout, err := os.Stdout.Stat(); err == nil && os.SameFile(info, stdout) {
		return os.Stdout, true
	}
	if stderr, err := os.Stderr.Stat(); err == nil && os.SameFile(info, stderr) {
		return os.Stderr, true
	}
	return nil, false
}

func resolveFormat(spec logHandlerSpec, writer io.Writer) log.Format {
	if spec.format.Valid() {
		return spec.format
	}
	if log.IsTerminal(writer) {
		return log.FormatText
	}
	return log.FormatJSON
}

func adjustLevel(base log.Level, verbose int) log.Level {
	_, hi := log.LevelRange()
	return min(base+log.Level(verbose), hi)
}

func closeLogHandlers(closers []io.Closer) error {
	var err error
	for _, closer := range closers {
		if closer != nil {
			err = errors.Join(err, closer.Close())
		}
	}
	return err
}
