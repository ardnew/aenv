package log_test

import (
	"context"
	"log/slog"
	"os"

	"github.com/ardnew/aenv/log"
)

func Example_basic() {
	logger := log.Make(os.Stdout)
	logger.Info("application started", slog.String("version", "1.0.0"))
}

func Example_configuration() {
	logger := log.Make(os.Stdout,
		log.WithLevel(log.LevelDebug),
		log.WithTimeLayout("RFC3339Nano"),
		log.WithCaller(true))

	logger.Debug("debug message with caller info")
}

func Example_levels() {
	logger := log.Make(os.Stdout, log.WithLevel(log.LevelWarn))

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message", slog.String("key", "value"))
	logger.Error("error message", slog.String("error", "something failed"))
}

func Example_textFormat() {
	logger := log.Make(os.Stdout, log.WithFormat(log.FormatText))
	logger.Info("text format message", slog.String("user", "alice"))
}

func Example_withAttributes() {
	// Create a logger with persistent attributes
	logger := log.Make(os.Stdout)
	logger = logger.With(slog.String("request_id", "12345"))

	logger.Info("processing request")
	logger.Debug("request details", slog.String("method", "GET"))
}

func Example_withContext() {
	type requestIDKey struct{}

	// Create a context with a request ID
	ctx := context.WithValue(context.Background(), requestIDKey{}, "req-789")

	logger := log.Make(os.Stdout)

	// Use context-aware logging methods
	logger.InfoContext(ctx, "processing request with context")
	logger.DebugContext(ctx, "request details", slog.String("method", "POST"))
}
