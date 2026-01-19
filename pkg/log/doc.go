// Package log provides a concurrency-safe simplified logging interface
// based on [log/slog].
//
// The package offers configurable time formatting, caller information,
// and output formats that are applied at logger creation time using
// functional options.
//
// # Basic Usage
//
//	logger := log.Make(os.Stdout)
//	logger.Info("application started", "version", "1.0.0")
//	logger.Error("failed to connect", "error", err)
//
// # Configuration
//
// Configure the logger using functional options:
//
//	logger := log.Make(os.Stdout,
//		log.WithLevel(log.LevelDebug),
//		log.WithTimeFormat("RFC3339Nano"),
//		log.WithCaller(true))
//
// # Adding Attributes
//
// Attributes can be added to the logger to be included in all subsequent
// log messages using the [Logger.With] method:
//
//	logger = logger.With("component", "api", "request_id", "123")
//	logger.Info("request received") // includes component=api request_id=123
//
// # Context-Aware Logging
//
// The package provides context-aware logging functions and methods.
// Each logging level has both a context-aware and context-unaware variant:
//
//	ctx := context.WithValue(context.Background(), "request-id", "12345")
//	logger.InfoContext(ctx, "processing request")
//	logger.Info("message without context") // uses DefaultContextProvider
//
// Context-unaware functions internally call their context-aware counterparts
// using [DefaultContextProvider], which returns [context.TODO] by default.
//
// # Supported Levels
//
// The package supports four log levels: [LevelDebug], [LevelInfo],
// [LevelWarn], and [LevelError]. Messages below the configured level
// are discarded.
//
// # Time Formatting
//
// Time formatting is configurable using [WithTimeLayout]. You can
// specify any named layout supported by the [time] package (such as
// "RFC3339" or "RFC3339Nano") or provide a custom layout string.
//
// # Output Formats
//
// Two output formats are supported: [FormatJSON] (default) and
// [FormatText]. Format is set at logger creation time using functional
// options.
package log
