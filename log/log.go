package log

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// Logger provides a concurrency-safe simplified logging interface.
type Logger struct {
	*slog.Logger
	config
}

// Make creates a new [Logger] that writes to the specified writer.
// The default configuration is [DefaultFormat], [DefaultLevel],
// [DefaultTimeLayout], and caller info disabled.
//
// Optional configuration can be applied using functional options like
// [WithFormat], [WithLevel], [WithTimeLayout], and [WithCaller].
func Make(w io.Writer, opts ...Option) Logger {
	cfg := makeConfig(w, opts...)

	// No need to lock the mutex here since we have the only reference to cfg.
	// The functional options will lock it as needed.

	return Logger{
		config: cfg,
		Logger: slog.New(cfg.handler()),
	}
}

// Wrap returns a new [Logger] that wraps the current logger with the provided
// configuration options.
// The existing configuration is used as the base, and any provided options
// will override specific values.
func (l Logger) Wrap(opts ...Option) Logger {
	// Method [config.clone] has a value receiver, implicitly copies l.config,
	// and creates a new mutex for the copy embedded in the returned Logger.
	//
	// By passings opts to [config.clone] — instead of [config.handler], below —
	// all of its mutations are performed when nothing else has a reference to
	// the new mutex.
	//
	// it has the only reference to the new mutex at that point.
	//
	// So [config.mutex] only needs to lock the [config.clone] call itself.
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	cfg := l.clone(opts...)

	return Logger{
		config: cfg,
		Logger: slog.New(cfg.handler()),
	}
}

// With returns a new [Logger] that includes the given attributes in each log
// message.
func (l Logger) With(attrs ...slog.Attr) Logger {
	if l.Logger == nil {
		return l
	}

	l.mutex.RLock()
	cfg := l.clone()
	l.mutex.RUnlock()

	return Logger{
		config: cfg,
		Logger: slog.New(l.Logger.Handler().WithAttrs(attrs)),
	}
}

// Level returns the current minimum log level.
func (l Logger) Level() Level {
	if l.Logger == nil {
		return DefaultLevel
	}

	if l.mutex == nil {
		l.mutex = &sync.RWMutex{}
	} else {
		l.mutex.RLock()
		defer l.mutex.RUnlock()
	}

	return l.level
}

// Format returns the current log output format.
func (l Logger) Format() Format {
	if l.Logger == nil {
		return DefaultFormat
	}

	if l.mutex == nil {
		l.mutex = &sync.RWMutex{}
	} else {
		l.mutex.RLock()
		defer l.mutex.RUnlock()
	}

	return l.format
}

// TraceContext logs a message at Trace level with the provided context.
func (l Logger) TraceContext(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logContext(ctx, LevelTrace, msg, attrs...)
}

// Trace logs a message at Trace level.
func (l Logger) Trace(msg string, attrs ...slog.Attr) {
	l.TraceContext(DefaultContextProvider(), msg, attrs...)
}

// DebugContext logs a message at Debug level with the provided context.
func (l Logger) DebugContext(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logContext(ctx, LevelDebug, msg, attrs...)
}

// Debug logs a message at Debug level.
func (l Logger) Debug(msg string, attrs ...slog.Attr) {
	l.DebugContext(DefaultContextProvider(), msg, attrs...)
}

// InfoContext logs a message at Info level with the provided context.
func (l Logger) InfoContext(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logContext(ctx, LevelInfo, msg, attrs...)
}

// Info logs a message at Info level.
func (l Logger) Info(msg string, attrs ...slog.Attr) {
	l.InfoContext(DefaultContextProvider(), msg, attrs...)
}

// WarnContext logs a message at Warn level with the provided context.
func (l Logger) WarnContext(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logContext(ctx, LevelWarn, msg, attrs...)
}

// Warn logs a message at Warn level.
func (l Logger) Warn(msg string, attrs ...slog.Attr) {
	l.WarnContext(DefaultContextProvider(), msg, attrs...)
}

// ErrorContext logs a message at Error level with the provided context.
func (l Logger) ErrorContext(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logContext(ctx, LevelError, msg, attrs...)
}

// Error logs a message at Error level.
func (l Logger) Error(msg string, attrs ...slog.Attr) {
	l.ErrorContext(DefaultContextProvider(), msg, attrs...)
}

// logContext writes a log message at the specified level with the provided
// context.
func (l Logger) logContext(
	ctx context.Context,
	level Level,
	msg string,
	attrs ...slog.Attr,
) {
	// Silently return for zero value loggers
	if l.Logger == nil {
		return
	}

	if l.mutex == nil {
		l.mutex = &sync.RWMutex{}
	} else {
		l.mutex.RLock()
		defer l.mutex.RUnlock()
	}

	// Use the lower-level log method to manually control the call stack skip.
	// We need to skip 4 frames to get to the actual caller:
	// 1. logContext (this function)
	// 2. TraceContext/DebugContext/InfoContext/WarnContext/ErrorContext
	// 3. Trace/Debug/Info/Warn/Error (optional - only for non-Context variants)
	// Since slog.Logger doesn't expose PC control directly, we use the Handler
	// interface with a custom Record that has the correct PC.
	if !l.Enabled(ctx, slog.Level(level)) {
		return
	}

	var pcs [1]uintptr
	// Skip 4 frames to get to actual caller:
	// 1=runtime.Callers, 2=logContext, 3=*Context method, 4=package-level wrapper
	runtime.Callers(4, pcs[:])

	r := slog.NewRecord(time.Now(), slog.Level(level), msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.Handler().Handle(ctx, r)
}
