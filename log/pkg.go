package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// DefaultContextProvider returns the default context used by context-unaware
// logging functions.
var DefaultContextProvider = context.TODO

// logStack is a linked list of attributed Logger configurations.
var defaultLog = Make(os.Stdout)

// Config updates the default logger with the given options.
func Config(opts ...Option) {
	defaultLog = defaultLog.Wrap(opts...)
}

// Raw writes the provided messages directly to the default logger's output
// with no formatting, attributes, or log metadata.
func Raw(msg ...string) {
	defaultLog.Raw(strings.Join(msg, " "))
}

// TraceContext logs a message at Trace level using the default logger with the
// provided context.
func TraceContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.logContext(ctx, LevelTrace, msg, attrs...)
}

// Trace logs a message at Trace level using the default logger.
func Trace(msg string, attrs ...slog.Attr) {
	defaultLog.logContext(DefaultContextProvider(), LevelTrace, msg, attrs...)
}

// DebugContext logs a message at Debug level using the default logger with the
// provided context.
func DebugContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.logContext(ctx, LevelDebug, msg, attrs...)
}

// Debug logs a message at Debug level using the default logger.
func Debug(msg string, attrs ...slog.Attr) {
	defaultLog.logContext(DefaultContextProvider(), LevelDebug, msg, attrs...)
}

// InfoContext logs a message at Info level using the default logger with the
// provided context.
func InfoContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.logContext(ctx, LevelInfo, msg, attrs...)
}

// Info logs a message at Info level using the default logger.
func Info(msg string, attrs ...slog.Attr) {
	defaultLog.logContext(DefaultContextProvider(), LevelInfo, msg, attrs...)
}

// WarnContext logs a message at Warn level using the default logger with the
// provided context.
func WarnContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.logContext(ctx, LevelWarn, msg, attrs...)
}

// Warn logs a message at Warn level using the default logger.
func Warn(msg string, attrs ...slog.Attr) {
	defaultLog.logContext(DefaultContextProvider(), LevelWarn, msg, attrs...)
}

// ErrorContext logs a message at Error level using the default logger with the
// provided context.
func ErrorContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.logContext(ctx, LevelError, msg, attrs...)
}

// Error logs a message at Error level using the default logger.
func Error(msg string, attrs ...slog.Attr) {
	defaultLog.logContext(DefaultContextProvider(), LevelError, msg, attrs...)
}

// With returns a new [Logger] that includes the given attributes in each log
// message using the default logger.
func With(attrs ...slog.Attr) Logger {
	return defaultLog.With(attrs...)
}

// Print writes only the provided attributes to the default logger's output
// using its configured format. No timestamp, level, message, or source
// information is included.
func Print(attrs ...slog.Attr) {
	defaultLog.Print(attrs...)
}

// CurrentLevel returns the current minimum log level of the default logger.
func CurrentLevel() Level {
	return defaultLog.Level()
}
