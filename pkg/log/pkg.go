package log

import (
	"context"
	"log/slog"
	"os"

	"github.com/ardnew/envcomp/pkg"
)

// DefaultContextProvider returns the default context used by context-unaware
// logging functions.
var DefaultContextProvider = context.TODO

// logStack is a linked list of attributed Logger configurations.
var defaultLog = Make(os.Stdout)

// Config updates the default logger with the given options.
func Config(opts ...pkg.Option[config]) {
	defaultLog = defaultLog.Wrap(opts...)
}

// DebugContext logs a message at Debug level using the default logger with the
// provided context.
func DebugContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.DebugContext(ctx, msg, attrs...)
}

// Debug logs a message at Debug level using the default logger.
func Debug(msg string, attrs ...slog.Attr) {
	DebugContext(DefaultContextProvider(), msg, attrs...)
}

// InfoContext logs a message at Info level using the default logger with the
// provided context.
func InfoContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.InfoContext(ctx, msg, attrs...)
}

// Info logs a message at Info level using the default logger.
func Info(msg string, attrs ...slog.Attr) {
	InfoContext(DefaultContextProvider(), msg, attrs...)
}

// WarnContext logs a message at Warn level using the default logger with the
// provided context.
func WarnContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.WarnContext(ctx, msg, attrs...)
}

// Warn logs a message at Warn level using the default logger.
func Warn(msg string, attrs ...slog.Attr) {
	WarnContext(DefaultContextProvider(), msg, attrs...)
}

// ErrorContext logs a message at Error level using the default logger with the
// provided context.
func ErrorContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	defaultLog.ErrorContext(ctx, msg, attrs...)
}

// Error logs a message at Error level using the default logger.
func Error(msg string, attrs ...slog.Attr) {
	ErrorContext(DefaultContextProvider(), msg, attrs...)
}

// With returns a new [Logger] that includes the given attributes in each log
// message using the default logger.
func With(attrs ...slog.Attr) Logger {
	return defaultLog.With(attrs...)
}
