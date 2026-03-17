package log

import (
	"context"
	"encoding/json"
	"fmt"
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

// Raw writes s directly to the configured output with no formatting,
// attributes, or log metadata. A trailing newline is appended if s does not
// already end with one. Output is always emitted regardless of the configured
// log level.
func (l Logger) Raw(s string) {
	if l.Logger == nil || len(s) == 0 {
		return
	}

	if l.mutex == nil {
		l.mutex = &sync.RWMutex{}
	}

	data := []byte(s)
	if s[len(s)-1] != '\n' {
		data = append(data, '\n')
	}

	l.mutex.Lock()
	_, _ = l.output.Write(data)
	l.mutex.Unlock()
}

// Print writes only the provided attributes to the configured output using
// the configured format. No timestamp, level, message, or source information
// is included. Output is always emitted regardless of the configured log level.
//
// For JSON format, attributes are rendered as a single JSON object.
// For text format, attributes are rendered as space-separated key=value pairs.
func (l Logger) Print(attrs ...slog.Attr) {
	if l.Logger == nil || len(attrs) == 0 {
		return
	}

	if l.mutex == nil {
		l.mutex = &sync.RWMutex{}
	}

	l.mutex.RLock()
	format := l.format
	pretty := l.pretty
	l.mutex.RUnlock()

	var data []byte

	switch format {
	case FormatJSON:
		data = printJSON(pretty, attrs)
	case FormatText:
		data = printText(pretty, attrs)
	default:
		return
	}

	l.mutex.Lock()
	_, _ = l.output.Write(data)
	l.mutex.Unlock()
}

// printJSON renders attributes as a JSON object.
func printJSON(pretty bool, attrs []slog.Attr) []byte {
	m := attrsToMap(attrs)

	data, err := marshalJSON(pretty, m)
	if err != nil {
		return []byte(err.Error() + "\n")
	}

	return append(data, '\n')
}

// marshalJSON marshals the value as JSON, optionally with indentation.
func marshalJSON(pretty bool, v any) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(v, "", "  ")
	}

	return json.Marshal(v)
}

// attrsToMap recursively converts [slog.Attr] values to a map, handling
// groups as nested maps.
func attrsToMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		if a.Value.Kind() == slog.KindGroup {
			m[a.Key] = attrsToMap(a.Value.Group())
		} else {
			m[a.Key] = a.Value.Any()
		}
	}

	return m
}

// printText renders attributes as space-separated key=value pairs.
func printText(_ bool, attrs []slog.Attr) []byte {
	var buf []byte

	for i, a := range attrs {
		if i > 0 {
			buf = append(buf, ' ')
		}

		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = appendTextValue(buf, a.Value)
	}

	return append(buf, '\n')
}

// appendTextValue appends the text representation of a [slog.Value] to buf.
func appendTextValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return append(buf, v.String()...)
	case slog.KindInt64:
		return fmt.Appendf(buf, "%d", v.Int64())
	case slog.KindUint64:
		return fmt.Appendf(buf, "%d", v.Uint64())
	case slog.KindFloat64:
		return fmt.Appendf(buf, "%g", v.Float64())
	case slog.KindBool:
		return fmt.Appendf(buf, "%t", v.Bool())
	case slog.KindDuration:
		return append(buf, v.Duration().String()...)
	case slog.KindTime:
		return append(buf, v.Time().Format(time.RFC3339Nano)...)
	case slog.KindGroup:
		buf = append(buf, '{')

		for i, a := range v.Group() {
			if i > 0 {
				buf = append(buf, ' ')
			}

			buf = append(buf, a.Key...)
			buf = append(buf, '=')
			buf = appendTextValue(buf, a.Value)
		}

		return append(buf, '}')
	case slog.KindAny, slog.KindLogValuer:
		return fmt.Appendf(buf, "%v", v.Any())
	}

	return buf
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
	l.logContext(DefaultContextProvider(), LevelTrace, msg, attrs...)
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
	l.logContext(DefaultContextProvider(), LevelDebug, msg, attrs...)
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
	l.logContext(DefaultContextProvider(), LevelInfo, msg, attrs...)
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
	l.logContext(DefaultContextProvider(), LevelWarn, msg, attrs...)
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
	l.logContext(DefaultContextProvider(), LevelError, msg, attrs...)
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

	// Use the lower-level Handler interface to manually control the call
	// stack skip. Both *Context and non-Context variants call logContext
	// directly, keeping the skip depth constant at 3:
	// 1=runtime.Callers, 2=logContext, 3=public logging method
	if !l.Enabled(ctx, slog.Level(level)) {
		return
	}

	// logContextCallerSkip is the number of stack frames to skip when
	// recording the call site. The 3 skipped frames are:
	// 1=runtime.Callers, 2=logContext, 3=public logging method.
	const logContextCallerSkip = 3

	var pcs [1]uintptr
	runtime.Callers(logContextCallerSkip, pcs[:])

	r := slog.NewRecord(time.Now(), slog.Level(level), msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.Handler().Handle(ctx, r)
}
