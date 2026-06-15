package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogger_Make_DefaultConfiguration(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	if logger.config.level != LevelInfo {
		t.Errorf("expected default level Info, got %v", logger.config.level)
	}
	if logger.config.callsite {
		t.Error("expected callsite disabled by default")
	}
	if logger.config.format != FormatJSON {
		t.Errorf("expected default format JSON, got %v", logger.config.format)
	}
}

func TestLogger_Make_WithLevel_FiltersMessages(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf, WithLevel(LevelDebug))

	logger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Error("debug message not logged after setting level to Debug")
	}

	buf.Reset()
	logger2 := Make(&buf, WithLevel(LevelError))
	logger2.Info("info message")
	if buf.Len() > 0 {
		t.Error("info message logged when level is Error")
	}

	logger2.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("error message not logged at Error level")
	}
}

func TestLogger_Make_WithTimeFormat_SetsLayout(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		contains string
	}{
		{"rfc3339 named", "RFC3339", "T"},
		{"rfc3339 nano named", "RFC3339Nano", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf, WithTimeLayout(tt.format), WithPretty(false))
			logger.Info("test")

			output := buf.String()
			if !strings.Contains(output, tt.contains) {
				t.Errorf(
					"expected time format to contain %q, got: %s",
					tt.contains,
					output,
				)
			}
		})
	}
}

func TestLogger_Make_WithCallsite_IncludesCallsiteInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf, WithCallsite(true))
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "source") {
		t.Error("callsite info not included when enabled")
	}

	buf.Reset()
	logger2 := Make(&buf, WithCallsite(false))
	logger2.Info("test message")

	output = buf.String()
	if strings.Contains(output, "source") {
		t.Error("callsite info included when disabled")
	}
}

func TestLogger_Make_WithFormat_SetsOutputFormat(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		logger := Make(&buf, WithFormat(FormatJSON), WithPretty(false))
		logger.Info("test message", slog.String("key", "value"))

		var result map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}
		if result["msg"] != "test message" {
			t.Errorf("expected msg=test message, got %v", result["msg"])
		}
		if result["key"] != "value" {
			t.Errorf("expected key=value, got %v", result["key"])
		}
	})

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		logger := Make(&buf, WithFormat(FormatText), WithPretty(false))
		logger.Info("test message", slog.String("key", "value"))

		output := buf.String()
		if !strings.Contains(output, "test message") {
			t.Error("message not found in text output")
		}
		if !strings.Contains(output, "key=value") {
			t.Error("key=value not found in text output")
		}
	})
}

func TestLogger_LogMethods_RespectLevelFiltering(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(Logger, string, ...slog.Attr)
		level    string
		minLevel Level
		logged   bool
	}{
		{"debug at debug", (Logger).Debug, "DEBUG", LevelDebug, true},
		{"debug at info", (Logger).Debug, "DEBUG", LevelInfo, false},
		{"info at info", (Logger).Info, "INFO", LevelInfo, true},
		{"info at warn", (Logger).Info, "INFO", LevelWarn, false},
		{"warn at warn", (Logger).Warn, "WARN", LevelWarn, true},
		{"warn at error", (Logger).Warn, "WARN", LevelError, false},
		{"error at error", (Logger).Error, "ERROR", LevelError, true},
		{"error at debug", (Logger).Error, "ERROR", LevelDebug, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf, WithLevel(tt.minLevel))
			tt.logFunc(logger, "test message")

			hasOutput := buf.Len() > 0
			if hasOutput != tt.logged {
				t.Errorf(
					"expected logged=%v, got output length=%d",
					tt.logged,
					buf.Len(),
				)
			}
		})
	}
}

func TestLogger_ConcurrentCalls_ThreadSafe(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf, WithPretty(false))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("concurrent message", slog.Int("id", id))
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 log lines, got %d", len(lines))
	}
}

func TestLogger_ConcurrentAccess_ThreadSafe(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			// Note: In functional options pattern, configuration
			// is set at creation time. This test now validates
			// concurrent logging rather than concurrent
			// reconfiguration.
			logger.Info("test message")
		}()
		go func() {
			defer wg.Done()
			logger.Info("test message")
		}()
	}
	wg.Wait()

	if buf.Len() == 0 {
		t.Error("no output generated during concurrent operations")
	}
}

func TestLogger_Make_MultipleOptions_AppliesAll(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelDebug),
		WithTimeLayout("RFC3339Nano"),
		WithCallsite(true),
		WithFormat(FormatText))

	logger.Debug("chained config test")

	output := buf.String()
	if !strings.Contains(output, "chained config test") {
		t.Error("message not logged after method chaining")
	}
}

func TestLogger_AllLevels_LogSuccessfully(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(Logger, string, ...slog.Attr)
		level   string
	}{
		{"trace", Logger.Trace, "trace"},
		{"debug", Logger.Debug, "debug"},
		{"info", Logger.Info, "info"},
		{"warn", Logger.Warn, "warn"},
		{"error", Logger.Error, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf, WithLevel(LevelTrace), WithFormat(FormatText), WithPretty(true))

			tt.logFunc(logger, "test message")

			output := buf.String()
			if !strings.Contains(output, "test message") {
				t.Errorf("expected %s message to be logged", tt.level)
			}
			// Verify the level string appears correctly (not"DEBUG-4" for trace)
			if !strings.Contains(output, tt.level) {
				t.Errorf("expected output to contain level %q, got: %s", tt.level, output)
			}
		})
	}
}

func TestLogger_Logging_ConcurrentCalls_ThreadSafe(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf, WithPretty(false))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("concurrent message", slog.Int("id", id))
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 log lines, got %d", len(lines))
	}
}

func TestLogger_LogsSuccessfully(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("expected message to be logged")
	}
}

func BenchmarkLogger_Info(b *testing.B) {
	var buf bytes.Buffer
	logger := Make(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", slog.Int("iteration", i))
	}
}

func BenchmarkLogger_Info_WithCallsite(b *testing.B) {
	var buf bytes.Buffer
	logger := Make(&buf, WithCallsite(true))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", slog.Int("iteration", i))
	}
}

func BenchmarkLogger_Info_WithAttributes(b *testing.B) {
	var buf bytes.Buffer
	logger := Make(&buf)
	logger = logger.With(slog.String("component", "test"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", slog.Int("iteration", i))
	}
}

func BenchmarkLogger_Info_Concurrent(b *testing.B) {
	var buf bytes.Buffer
	logger := Make(&buf)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("concurrent message", slog.Int("id", i))
			i++
		}
	})
}

func TestLogger_With_AddsAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf, WithFormat(FormatJSON), WithPretty(false))

	loggerWith := logger.With(slog.String("key", "value"))
	loggerWith.Info("test message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if val, ok := entry["key"]; !ok || val != "value" {
		t.Errorf("expected key=value in log entry, got %v", val)
	}
}

func TestLogger_ZeroValue_Safety(t *testing.T) {
	var l Logger
	// Should not panic
	l.Debug("test")
	l.Info("test")
	l.Warn("test")
	l.Error("test")

	l2 := l.With(slog.String("key", "value"))
	if l2.Logger != nil {
		t.Error("expected nil logger from zero value With")
	}
}

func TestLogger_EdgeCases(t *testing.T) {
	// Test empty time format
	var buf bytes.Buffer
	l := Make(&buf, WithTimeLayout("none"), WithPretty(false)) // "none" maps to ""
	l.Info("test")
	output := buf.String()
	// With empty time layout, the time field should be omitted by ReplaceAttr
	if strings.Contains(output, `"time"`) {
		t.Errorf("expected no time field, got: %s", output)
	}
}

func TestLogger_ContextMethods_LogSuccessfully(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(Logger, string, ...slog.Attr)
		level   string
	}{
		{"debug", func(l Logger, msg string, attrs ...slog.Attr) {
			l.DebugContext(DefaultContextProvider(), msg, attrs...)
		}, "debug"},
		{"info", func(l Logger, msg string, attrs ...slog.Attr) {
			l.InfoContext(DefaultContextProvider(), msg, attrs...)
		}, "info"},
		{"warn", func(l Logger, msg string, attrs ...slog.Attr) {
			l.WarnContext(DefaultContextProvider(), msg, attrs...)
		}, "warn"},
		{"error", func(l Logger, msg string, attrs ...slog.Attr) {
			l.ErrorContext(DefaultContextProvider(), msg, attrs...)
		}, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf, WithLevel(LevelDebug))

			tt.logFunc(logger, "test message")

			output := buf.String()
			if !strings.Contains(output, "test message") {
				t.Errorf("expected %s message to be logged", tt.level)
			}
		})
	}
}

func TestPackage_ContextFunctions_UseDefaultLogger(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(string, ...slog.Attr)
	}{
		{"DebugContext", func(msg string, attrs ...slog.Attr) {
			DebugContext(DefaultContextProvider(), msg, attrs...)
		}},
		{"InfoContext", func(msg string, attrs ...slog.Attr) {
			InfoContext(DefaultContextProvider(), msg, attrs...)
		}},
		{"WarnContext", func(msg string, attrs ...slog.Attr) {
			WarnContext(DefaultContextProvider(), msg, attrs...)
		}},
		{"ErrorContext", func(msg string, attrs ...slog.Attr) {
			ErrorContext(DefaultContextProvider(), msg, attrs...)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Config(WithOutput(&buf), WithLevel(LevelDebug))

			tt.logFunc("package context test")

			output := buf.String()
			if !strings.Contains(output, "package context test") {
				t.Error("expected message to be logged using package context function")
			}
		})
	}
}

// sourceEntry represents the source location embedded in a JSON log entry.
type sourceEntry struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// logEntry represents a JSON log entry with source information.
type logEntry struct {
	Source sourceEntry `json:"source"`
}

// parseSource extracts the source location from a JSON log line.
func parseSource(t *testing.T, output string) sourceEntry {
	t.Helper()

	var entry logEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v\noutput: %s", err, output)
	}

	return entry.Source
}

func TestLogger_Callsite_NonContextMethods_ReportCaller(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)

	tests := []struct {
		name    string
		logFunc func(Logger, string, ...slog.Attr)
	}{
		{"Trace", Logger.Trace},
		{"Debug", Logger.Debug},
		{"Info", Logger.Info},
		{"Warn", Logger.Warn},
		{"Error", Logger.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf,
				WithLevel(LevelTrace),
				WithCallsite(true),
				WithFormat(FormatJSON),
				WithPretty(false),
			)

			tt.logFunc(logger, "caller test")
			src := parseSource(t, buf.String())

			if src.File != thisFile {
				t.Errorf(
					"expected source file %q, got %q",
					thisFile, src.File,
				)
			}
			if src.Function == "" {
				t.Error("expected non-empty source function")
			}
		})
	}
}

func TestLogger_Callsite_ContextMethods_ReportCaller(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	ctx := context.Background()

	tests := []struct {
		name    string
		logFunc func(Logger, context.Context, string, ...slog.Attr)
	}{
		{"TraceContext", Logger.TraceContext},
		{"DebugContext", Logger.DebugContext},
		{"InfoContext", Logger.InfoContext},
		{"WarnContext", Logger.WarnContext},
		{"ErrorContext", Logger.ErrorContext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf,
				WithLevel(LevelTrace),
				WithCallsite(true),
				WithFormat(FormatJSON),
				WithPretty(false),
			)

			tt.logFunc(logger, ctx, "caller test")
			src := parseSource(t, buf.String())

			if src.File != thisFile {
				t.Errorf(
					"expected source file %q, got %q",
					thisFile, src.File,
				)
			}
			if src.Function == "" {
				t.Error("expected non-empty source function")
			}
		})
	}
}

func TestLogger_Callsite_BothVariants_ReportSameFile(t *testing.T) {
	var ctxBuf, plainBuf bytes.Buffer
	ctx := context.Background()

	ctxLogger := Make(&ctxBuf,
		WithLevel(LevelTrace),
		WithCallsite(true),
		WithFormat(FormatJSON),
		WithPretty(false),
	)
	plainLogger := Make(&plainBuf,
		WithLevel(LevelTrace),
		WithCallsite(true),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	type variant struct {
		name      string
		callCtx   func()
		callPlain func()
		ctxBuf    *bytes.Buffer
		plainBuf  *bytes.Buffer
	}

	variants := []variant{
		{
			"Info",
			func() { ctxLogger.InfoContext(ctx, "ctx") },
			func() { plainLogger.Info("plain") },
			&ctxBuf, &plainBuf,
		},
		{
			"Error",
			func() { ctxLogger.ErrorContext(ctx, "ctx") },
			func() { plainLogger.Error("plain") },
			&ctxBuf, &plainBuf,
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			v.ctxBuf.Reset()
			v.plainBuf.Reset()

			v.callCtx()
			v.callPlain()

			ctxSrc := parseSource(t, v.ctxBuf.String())
			plainSrc := parseSource(t, v.plainBuf.String())

			if ctxSrc.File != plainSrc.File {
				t.Errorf(
					"source file mismatch: Context=%q, plain=%q",
					ctxSrc.File, plainSrc.File,
				)
			}
			// Both should point to this test file
			_, thisFile, _, _ := runtime.Caller(0)
			if ctxSrc.File != thisFile {
				t.Errorf(
					"expected caller file %q, got %q",
					thisFile, ctxSrc.File,
				)
			}
		})
	}
}

func TestLogger_Callsite_ReportsCorrectLine(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelTrace),
		WithCallsite(true),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	_, _, expectedLine, _ := runtime.Caller(0)
	expectedLine += 2 // skip this line; the log call is two lines below
	logger.Info("line test")

	src := parseSource(t, buf.String())

	if src.Line != expectedLine {
		t.Errorf("expected line %d, got %d", expectedLine, src.Line)
	}

	// Same for Context variant
	buf.Reset()
	_, _, expectedLine, _ = runtime.Caller(0)
	expectedLine += 2
	logger.InfoContext(context.Background(), "line test ctx")

	src = parseSource(t, buf.String())

	if src.Line != expectedLine {
		t.Errorf(
			"expected line %d for Context variant, got %d",
			expectedLine, src.Line,
		)
	}
}

// helperThatLogs calls a logger method from a helper function to verify that
// the caller reported is the helper, not what called the helper.
func helperThatLogs(logger Logger, msg string) (file string, line int) {
	_, file, line, _ = runtime.Caller(0)
	line += 2 // skip this line; the log call is two lines below
	logger.Info(msg)

	return file, line
}

func TestLogger_Callsite_ReportsImmediateCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelTrace),
		WithCallsite(true),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	expectedFile, expectedLine := helperThatLogs(logger, "helper call")
	src := parseSource(t, buf.String())

	if src.File != expectedFile {
		t.Errorf("expected file %q, got %q", expectedFile, src.File)
	}
	if src.Line != expectedLine {
		t.Errorf("expected line %d, got %d", expectedLine, src.Line)
	}

	// The source should NOT point to this test function
	_, thisFile, thisLine, _ := runtime.Caller(0)
	if src.File == thisFile && src.Line >= thisLine-10 && src.Line <= thisLine {
		t.Error("source should point to helper, not to this test function")
	}
}

func TestLogger_Callsite_DoesNotReportInternalLogPackage(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelTrace),
		WithCallsite(true),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	methods := []struct {
		name string
		call func()
	}{
		{"Info", func() { logger.Info("test") }},
		{"InfoContext", func() {
			logger.InfoContext(context.Background(), "test")
		}},
		{"Error", func() { logger.Error("test") }},
		{"ErrorContext", func() {
			logger.ErrorContext(context.Background(), "test")
		}},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			buf.Reset()
			m.call()

			src := parseSource(t, buf.String())

			// Source must never point to log.go (internal implementation)
			if strings.HasSuffix(src.File, "/log.go") {
				t.Errorf(
					"%s: source points to internal log.go (%s:%d), "+
						"expected caller location",
					m.name, src.File, src.Line,
				)
			}
			if strings.Contains(src.Function, ".logContext") {
				t.Errorf(
					"%s: source function is internal logContext: %s",
					m.name, src.Function,
				)
			}
		})
	}
}

func TestLogger_Callsite_AllMethodPairs_ConsistentDepth(t *testing.T) {
	// Each call function logs and returns the line number of the log call.
	makePlainCall := func(
		logFn func(Logger, string, ...slog.Attr),
	) func(Logger, *bytes.Buffer) int {
		return func(l Logger, buf *bytes.Buffer) int {
			buf.Reset()
			_, _, line, _ := runtime.Caller(0)
			logFn(l, "test") // line+1
			return line + 1
		}
	}
	makeCtxCall := func(
		logFn func(Logger, context.Context, string, ...slog.Attr),
	) func(Logger, *bytes.Buffer) int {
		return func(l Logger, buf *bytes.Buffer) int {
			buf.Reset()
			_, _, line, _ := runtime.Caller(0)
			logFn(l, context.Background(), "test") // line+1
			return line + 1
		}
	}

	type methodPair struct {
		name      string
		callPlain func(Logger, *bytes.Buffer) int
		callCtx   func(Logger, *bytes.Buffer) int
	}

	pairs := []methodPair{
		{"Trace", makePlainCall(Logger.Trace), makeCtxCall(Logger.TraceContext)},
		{"Debug", makePlainCall(Logger.Debug), makeCtxCall(Logger.DebugContext)},
		{"Info", makePlainCall(Logger.Info), makeCtxCall(Logger.InfoContext)},
		{"Warn", makePlainCall(Logger.Warn), makeCtxCall(Logger.WarnContext)},
		{"Error", makePlainCall(Logger.Error), makeCtxCall(Logger.ErrorContext)},
	}

	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelTrace),
		WithCallsite(true),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			plainLine := pair.callPlain(logger, &buf)
			plainSrc := parseSource(t, buf.String())

			ctxLine := pair.callCtx(logger, &buf)
			ctxSrc := parseSource(t, buf.String())

			if plainSrc.Line != plainLine {
				t.Errorf(
					"plain variant: expected line %d, got %d",
					plainLine, plainSrc.Line,
				)
			}
			if ctxSrc.Line != ctxLine {
				t.Errorf(
					"Context variant: expected line %d, got %d",
					ctxLine, ctxSrc.Line,
				)
			}

			// Both must report the same file
			if plainSrc.File != ctxSrc.File {
				t.Errorf(
					"file mismatch: plain=%q, Context=%q",
					plainSrc.File, ctxSrc.File,
				)
			}

			// Verify neither points to internal implementation
			for _, src := range []sourceEntry{plainSrc, ctxSrc} {
				if strings.HasSuffix(src.File, "/log.go") {
					t.Errorf(
						"source points to internal log.go: %s:%d",
						src.File, src.Line,
					)
				}
			}
		})
	}
}

// parseJSON unmarshals a JSON log line into a generic map.
func parseJSON(t *testing.T, output string) map[string]any {
	t.Helper()

	var entry map[string]any
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v\noutput: %s", err, output)
	}

	return entry
}

// newJSONLogger creates a Logger writing non-pretty JSON at the given level.
func newJSONLogger(buf *bytes.Buffer, level Level) Logger {
	return Make(buf,
		WithLevel(level),
		WithFormat(FormatJSON),
		WithPretty(false),
	)
}

func TestLogger_JSONEntry_ContainsStandardFields(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Info("standard fields")
	entry := parseJSON(t, buf.String())

	// time must be present and non-empty
	timeVal, ok := entry["time"]
	if !ok {
		t.Fatal("expected 'time' field in JSON output")
	}
	if timeStr, ok := timeVal.(string); !ok || timeStr == "" {
		t.Errorf("expected non-empty string time, got %v", timeVal)
	}

	// level must be present
	levelVal, ok := entry["level"]
	if !ok {
		t.Fatal("expected 'level' field in JSON output")
	}
	if levelStr, ok := levelVal.(string); !ok || levelStr != "INFO" {
		t.Errorf("expected level=INFO, got %v", levelVal)
	}

	// msg must be present
	msgVal, ok := entry["msg"]
	if !ok {
		t.Fatal("expected 'msg' field in JSON output")
	}
	if msgStr, ok := msgVal.(string); !ok || msgStr != "standard fields" {
		t.Errorf("expected msg='standard fields', got %v", msgVal)
	}

	// source must NOT be present (callsite disabled by default)
	if _, ok := entry["source"]; ok {
		t.Error("expected no 'source' field when callsite is disabled")
	}
}

func TestLogger_JSONEntry_LevelValues_AllLevels(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelTrace, "TRACE"},
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newJSONLogger(&buf, LevelTrace)

			logger.logContext(
				context.Background(), tt.level, "level test",
			)
			entry := parseJSON(t, buf.String())

			levelVal, ok := entry["level"].(string)
			if !ok {
				t.Fatalf("level field missing or not string: %v", entry["level"])
			}
			if levelVal != tt.expected {
				t.Errorf("expected level %q, got %q", tt.expected, levelVal)
			}
		})
	}
}

func TestLogger_JSONEntry_TimeField_Parseable(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelInfo),
		WithFormat(FormatJSON),
		WithPretty(false),
		WithTimeLayout("RFC3339Nano"),
	)

	before := time.Now()
	logger.Info("time test")
	after := time.Now()

	entry := parseJSON(t, buf.String())

	timeStr, ok := entry["time"].(string)
	if !ok {
		t.Fatalf("time field missing or not string: %v", entry["time"])
	}

	parsed, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", timeStr, err)
	}

	if parsed.Before(before) || parsed.After(after) {
		t.Errorf(
			"time %v not between %v and %v",
			parsed, before, after,
		)
	}
}

func TestLogger_JSONEntry_NoTimeField_WhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatJSON),
		WithPretty(false),
		WithTimeLayout("none"),
	)

	logger.Info("no time")
	entry := parseJSON(t, buf.String())

	if _, ok := entry["time"]; ok {
		t.Error("expected no time field when layout is 'none'")
	}
}

func TestLogger_JSONEntry_AttributeTypes(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		attr     slog.Attr
		key      string
		validate func(t *testing.T, val any)
	}{
		{
			"string",
			slog.String("str", "hello"),
			"str",
			func(t *testing.T, val any) {
				t.Helper()
				if v, ok := val.(string); !ok || v != "hello" {
					t.Errorf("expected string 'hello', got %v", val)
				}
			},
		},
		{
			"int",
			slog.Int("num", 42),
			"num",
			func(t *testing.T, val any) {
				t.Helper()
				if v, ok := val.(float64); !ok || v != 42 {
					t.Errorf("expected int 42, got %v", val)
				}
			},
		},
		{
			"int64",
			slog.Int64("big", 9999999999),
			"big",
			func(t *testing.T, val any) {
				t.Helper()
				if v, ok := val.(float64); !ok || v != 9999999999 {
					t.Errorf("expected int64 9999999999, got %v", val)
				}
			},
		},
		{
			"float64",
			slog.Float64("pi", 3.14159),
			"pi",
			func(t *testing.T, val any) {
				t.Helper()
				v, ok := val.(float64)
				if !ok {
					t.Fatalf("expected float64, got %T", val)
				}
				if v < 3.141 || v > 3.142 {
					t.Errorf("expected ~3.14159, got %v", v)
				}
			},
		},
		{
			"bool true",
			slog.Bool("flag", true),
			"flag",
			func(t *testing.T, val any) {
				t.Helper()
				if v, ok := val.(bool); !ok || !v {
					t.Errorf("expected bool true, got %v", val)
				}
			},
		},
		{
			"bool false",
			slog.Bool("off", false),
			"off",
			func(t *testing.T, val any) {
				t.Helper()
				if v, ok := val.(bool); !ok || v {
					t.Errorf("expected bool false, got %v", val)
				}
			},
		},
		{
			"duration",
			slog.Duration("elapsed", 2*time.Second+500*time.Millisecond),
			"elapsed",
			func(t *testing.T, val any) {
				t.Helper()
				// slog encodes Duration as nanoseconds (float64 in JSON)
				v, ok := val.(float64)
				if !ok {
					t.Fatalf("expected float64, got %T", val)
				}
				expected := float64(2*time.Second + 500*time.Millisecond)
				if v != expected {
					t.Errorf("expected duration %v, got %v", expected, v)
				}
			},
		},
		{
			"time",
			slog.Time("when", now),
			"when",
			func(t *testing.T, val any) {
				t.Helper()
				v, ok := val.(string)
				if !ok {
					t.Fatalf("expected string, got %T", val)
				}
				parsed, err := time.Parse(time.RFC3339Nano, v)
				if err != nil {
					t.Fatalf("failed to parse time attr: %v", err)
				}
				if !parsed.Equal(now) {
					t.Errorf("expected %v, got %v", now, parsed)
				}
			},
		},
		{
			"any",
			slog.Any("data", []int{1, 2, 3}),
			"data",
			func(t *testing.T, val any) {
				t.Helper()
				arr, ok := val.([]any)
				if !ok {
					t.Fatalf("expected array, got %T", val)
				}
				if len(arr) != 3 {
					t.Errorf("expected 3 elements, got %d", len(arr))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newJSONLogger(&buf, LevelInfo)

			logger.Info("attr test", tt.attr)
			entry := parseJSON(t, buf.String())

			val, ok := entry[tt.key]
			if !ok {
				t.Fatalf("attribute %q not found in output", tt.key)
			}
			tt.validate(t, val)
		})
	}
}

func TestLogger_JSONEntry_GroupAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Info("group test",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.Int("status", 200),
		),
	)

	entry := parseJSON(t, buf.String())

	group, ok := entry["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'request' group as object, got %T", entry["request"])
	}

	if group["method"] != "GET" {
		t.Errorf("expected method=GET, got %v", group["method"])
	}
	if status, ok := group["status"].(float64); !ok || status != 200 {
		t.Errorf("expected status=200, got %v", group["status"])
	}
}

func TestLogger_JSONEntry_MultipleAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Info("multi attrs",
		slog.String("user", "alice"),
		slog.Int("age", 30),
		slog.Bool("admin", true),
		slog.Float64("score", 99.5),
	)

	entry := parseJSON(t, buf.String())

	expected := map[string]any{
		"user":  "alice",
		"age":   float64(30),
		"admin": true,
		"score": 99.5,
	}

	for key, want := range expected {
		got, ok := entry[key]
		if !ok {
			t.Errorf("missing attribute %q", key)

			continue
		}
		if got != want {
			t.Errorf("attribute %q: expected %v (%T), got %v (%T)",
				key, want, want, got, got)
		}
	}
}

func TestLogger_JSONEntry_WithPlusInlineAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	loggerWith := logger.With(
		slog.String("service", "api"),
		slog.Int("pid", 1234),
	)

	loggerWith.Info("combined attrs",
		slog.String("action", "login"),
		slog.Bool("success", true),
	)

	entry := parseJSON(t, buf.String())

	// Persistent With attrs
	if entry["service"] != "api" {
		t.Errorf("expected service=api, got %v", entry["service"])
	}
	if pid, ok := entry["pid"].(float64); !ok || pid != 1234 {
		t.Errorf("expected pid=1234, got %v", entry["pid"])
	}

	// Inline attrs
	if entry["action"] != "login" {
		t.Errorf("expected action=login, got %v", entry["action"])
	}
	if entry["success"] != true {
		t.Errorf("expected success=true, got %v", entry["success"])
	}
}

func TestLogger_JSONEntry_WithChaining_AccumulatesAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	l1 := logger.With(slog.String("a", "1"))
	l2 := l1.With(slog.String("b", "2"))

	l2.Info("chained")
	entry := parseJSON(t, buf.String())

	if entry["a"] != "1" {
		t.Errorf("expected a=1, got %v", entry["a"])
	}
	if entry["b"] != "2" {
		t.Errorf("expected b=2, got %v", entry["b"])
	}
}

func TestLogger_JSONEntry_EmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Info("")
	entry := parseJSON(t, buf.String())

	msg, ok := entry["msg"].(string)
	if !ok {
		t.Fatal("expected msg field")
	}
	if msg != "" {
		t.Errorf("expected empty msg, got %q", msg)
	}
}

func TestLogger_JSONEntry_NoExtraFields_Without_Attributes(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Info("bare")
	entry := parseJSON(t, buf.String())

	allowedKeys := map[string]bool{
		"time":  true,
		"level": true,
		"msg":   true,
	}

	for key := range entry {
		if !allowedKeys[key] {
			t.Errorf("unexpected field %q in bare log entry", key)
		}
	}
}

func TestLogger_TextFormat_ContainsAllAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelInfo),
		WithFormat(FormatText),
		WithPretty(false),
	)

	logger.Info("text test",
		slog.String("user", "bob"),
		slog.Int("count", 7),
		slog.Bool("active", true),
	)

	output := buf.String()

	expected := []string{
		"text test",
		"user=bob",
		"count=7",
		"active=true",
	}

	for _, s := range expected {
		if !strings.Contains(output, s) {
			t.Errorf("expected text output to contain %q, got: %s", s, output)
		}
	}
}

func TestLogger_TextFormat_ContainsLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelTrace, "TRACE"},
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf,
				WithLevel(LevelTrace),
				WithFormat(FormatText),
				WithPretty(false),
			)

			logger.logContext(
				context.Background(), tt.level, "level text",
			)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf(
					"expected text output to contain %q, got: %s",
					tt.expected, output,
				)
			}
		})
	}
}

func TestLogger_Wrap_OverridesOptions(t *testing.T) {
	var buf bytes.Buffer
	base := Make(&buf,
		WithLevel(LevelInfo),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	// Wrap to a new level
	wrapped := base.Wrap(WithLevel(LevelDebug))

	// Base should NOT log debug
	base.Debug("base debug")
	if buf.Len() > 0 {
		t.Error("base logger should not log debug after Wrap")
	}

	// Wrapped should log debug
	var buf2 bytes.Buffer
	wrapped2 := base.Wrap(
		WithLevel(LevelDebug),
		WithOutput(&buf2),
	)
	wrapped2.Debug("wrapped debug")
	if buf2.Len() == 0 {
		t.Error("wrapped logger should log debug")
	}

	// Verify the wrapped logger reflects new level
	if wrapped.Level() != LevelDebug {
		t.Errorf("expected wrapped level Debug, got %v", wrapped.Level())
	}
}

func TestLogger_Wrap_PreservesBaseConfig(t *testing.T) {
	var baseBuf bytes.Buffer
	base := Make(&baseBuf,
		WithLevel(LevelWarn),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	// Wrap only changes level
	var wrappedBuf bytes.Buffer
	wrapped := base.Wrap(
		WithLevel(LevelDebug),
		WithOutput(&wrappedBuf),
	)

	if wrapped.Format() != FormatJSON {
		t.Errorf(
			"expected wrapped format to inherit JSON, got %v",
			wrapped.Format(),
		)
	}

	// Base config unaffected
	if base.Level() != LevelWarn {
		t.Errorf("expected base level Warn, got %v", base.Level())
	}
}

func TestLogger_Wrap_ChangesFormat(t *testing.T) {
	var buf bytes.Buffer
	base := Make(&buf,
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	var textBuf bytes.Buffer
	text := base.Wrap(
		WithFormat(FormatText),
		WithOutput(&textBuf),
		WithPretty(false),
	)

	text.Info("format test", slog.String("k", "v"))

	output := textBuf.String()
	if !strings.Contains(output, "k=v") {
		t.Errorf("expected text format output with k=v, got: %s", output)
	}

	// Should NOT be valid JSON
	var discard map[string]any
	if err := json.Unmarshal(textBuf.Bytes(), &discard); err == nil {
		t.Error("text format output should not be valid JSON")
	}
}

func TestLogger_Level_ReturnsConfiguredLevel(t *testing.T) {
	tests := []struct {
		name  string
		level Level
	}{
		{"trace", LevelTrace},
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf, WithLevel(tt.level))

			if logger.Level() != tt.level {
				t.Errorf(
					"expected Level()=%v, got %v",
					tt.level, logger.Level(),
				)
			}
		})
	}
}

func TestLogger_Level_ZeroValue_ReturnsDefault(t *testing.T) {
	var l Logger

	if l.Level() != DefaultLevel {
		t.Errorf("expected zero-value Level()=%v, got %v", DefaultLevel, l.Level())
	}
}

func TestLogger_Format_ReturnsConfiguredFormat(t *testing.T) {
	tests := []struct {
		name   string
		format Format
	}{
		{"json", FormatJSON},
		{"text", FormatText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := Make(&buf, WithFormat(tt.format))

			if logger.Format() != tt.format {
				t.Errorf(
					"expected Format()=%v, got %v",
					tt.format, logger.Format(),
				)
			}
		})
	}
}

func TestLogger_Format_ZeroValue_ReturnsDefault(t *testing.T) {
	var l Logger

	if l.Format() != DefaultFormat {
		t.Errorf(
			"expected zero-value Format()=%v, got %v",
			DefaultFormat, l.Format(),
		)
	}
}

func TestLogger_JSONEntry_SourceStructure_WithCallsite(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelInfo),
		WithFormat(FormatJSON),
		WithPretty(false),
		WithCallsite(true),
	)

	logger.Info("source structure")
	entry := parseJSON(t, buf.String())

	srcRaw, ok := entry["source"]
	if !ok {
		t.Fatal("expected 'source' field when callsite enabled")
	}

	src, ok := srcRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected source as object, got %T", srcRaw)
	}

	// Verify all source sub-fields are present
	for _, field := range []string{"function", "file", "line"} {
		if _, ok := src[field]; !ok {
			t.Errorf("expected source.%s field", field)
		}
	}

	// function should contain the test function name
	fn, _ := src["function"].(string)
	if !strings.Contains(fn, "TestLogger_JSONEntry_SourceStructure_WithCallsite") {
		t.Errorf(
			"expected source.function to contain test name, got %q",
			fn,
		)
	}

	// line should be positive
	line, _ := src["line"].(float64)
	if line <= 0 {
		t.Errorf("expected positive source.line, got %v", line)
	}
}

func TestLogger_With_DoesNotMutateParent(t *testing.T) {
	var parentBuf, childBuf bytes.Buffer
	parent := newJSONLogger(&parentBuf, LevelInfo)

	child := parent.With(slog.String("child_key", "child_val"))

	// Use a separate buffer for the child
	childWithBuf := Make(&childBuf,
		WithLevel(LevelInfo),
		WithFormat(FormatJSON),
		WithPretty(false),
	).With(slog.String("child_key", "child_val"))

	// Parent should not have child's attribute
	parentBuf.Reset()
	parent.Info("parent log")
	parentEntry := parseJSON(t, parentBuf.String())

	if _, ok := parentEntry["child_key"]; ok {
		t.Error("parent should not have child's attribute")
	}

	// Child should have the attribute
	childBuf.Reset()
	childWithBuf.Info("child log")
	childEntry := parseJSON(t, childBuf.String())

	if childEntry["child_key"] != "child_val" {
		t.Errorf(
			"expected child_key=child_val, got %v",
			childEntry["child_key"],
		)
	}

	// Verify child reference is separate from parent
	_ = child // ensure no unused variable
}

func TestLogger_Wrap_Callsite_ReportsCorrectCaller(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)

	var baseBuf bytes.Buffer
	base := Make(&baseBuf,
		WithLevel(LevelInfo),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	var wrappedBuf bytes.Buffer
	wrapped := base.Wrap(
		WithCallsite(true),
		WithOutput(&wrappedBuf),
	)

	wrapped.Info("wrap callsite")
	src := parseSource(t, wrappedBuf.String())

	if src.File != thisFile {
		t.Errorf("expected source file %q, got %q", thisFile, src.File)
	}
}

func TestLogger_Print_JSON_RendersOnlyAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Print(
		slog.String("name", "aenv"),
		slog.String("version", "1.0.0"),
	)

	entry := parseJSON(t, buf.String())

	// Must contain attributes
	if entry["name"] != "aenv" {
		t.Errorf("expected name=aenv, got %v", entry["name"])
	}
	if entry["version"] != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %v", entry["version"])
	}

	// Must NOT contain standard log fields
	for _, key := range []string{"time", "level", "msg", "source"} {
		if _, ok := entry[key]; ok {
			t.Errorf("expected no %q field in Print output", key)
		}
	}
}

func TestLogger_Print_Text_RendersOnlyAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatText),
		WithPretty(false),
	)

	logger.Print(
		slog.String("name", "aenv"),
		slog.Int("count", 42),
	)

	output := buf.String()

	if !strings.Contains(output, "name=aenv") {
		t.Errorf("expected name=aenv in output, got: %s", output)
	}
	if !strings.Contains(output, "count=42") {
		t.Errorf("expected count=42 in output, got: %s", output)
	}

	// Must not contain level or time markers
	for _, s := range []string{"INFO", "DEBUG", "WARN", "ERROR", "time="} {
		if strings.Contains(output, s) {
			t.Errorf("expected no %q in Print text output, got: %s", s, output)
		}
	}
}

func TestLogger_Print_JSON_AllAttributeTypes(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Print(
		slog.String("s", "hello"),
		slog.Int("i", 42),
		slog.Float64("f", 3.14),
		slog.Bool("b", true),
		slog.Time("t", now),
		slog.Duration("d", 5*time.Second),
	)

	entry := parseJSON(t, buf.String())

	if entry["s"] != "hello" {
		t.Errorf("expected s=hello, got %v", entry["s"])
	}
	if v, ok := entry["i"].(float64); !ok || v != 42 {
		t.Errorf("expected i=42, got %v", entry["i"])
	}
	if v, ok := entry["f"].(float64); !ok || v < 3.13 || v > 3.15 {
		t.Errorf("expected f~3.14, got %v", entry["f"])
	}
	if entry["b"] != true {
		t.Errorf("expected b=true, got %v", entry["b"])
	}
	if _, ok := entry["t"]; !ok {
		t.Error("expected t field present")
	}
	if _, ok := entry["d"]; !ok {
		t.Error("expected d field present")
	}
}

func TestLogger_Print_Text_AllAttributeTypes(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatText),
		WithPretty(false),
	)

	logger.Print(
		slog.String("s", "hello"),
		slog.Int("i", 42),
		slog.Float64("f", 3.14),
		slog.Bool("b", true),
		slog.Duration("d", 5*time.Second),
	)

	output := buf.String()

	expected := []string{
		"s=hello",
		"i=42",
		"f=3.14",
		"b=true",
		"d=5s",
	}

	for _, s := range expected {
		if !strings.Contains(output, s) {
			t.Errorf("expected %q in text output, got: %s", s, output)
		}
	}
}

func TestLogger_Print_EmptyAttrs_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Print()

	if buf.Len() > 0 {
		t.Errorf("expected no output for empty Print, got: %s", buf.String())
	}
}

func TestLogger_Print_ZeroValue_NoOutput(t *testing.T) {
	var l Logger
	// Should not panic
	l.Print(slog.String("key", "val"))
}

func TestLogger_Print_IgnoresLogLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithLevel(LevelError),
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	logger.Print(slog.String("key", "val"))

	if buf.Len() == 0 {
		t.Error("Print should emit output regardless of configured log level")
	}
}

func TestLogger_Print_JSON_PrettyFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatJSON),
		WithPretty(true),
	)

	logger.Print(slog.String("key", "val"))

	output := buf.String()

	// Pretty JSON should have indentation
	if !strings.Contains(output, "  ") {
		t.Errorf("expected indented JSON in pretty mode, got: %s", output)
	}
}

func TestLogger_Print_JSON_NonPrettyCompact(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	logger.Print(slog.String("key", "val"))

	output := buf.String()

	// Non-pretty JSON should be a single line
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("expected single-line JSON, got %d lines: %s", len(lines), output)
	}
}

func TestLogger_Print_SingleAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Print(slog.String("version", "2.0"))

	entry := parseJSON(t, buf.String())

	if len(entry) != 1 {
		t.Errorf("expected exactly 1 field, got %d: %v", len(entry), entry)
	}
	if entry["version"] != "2.0" {
		t.Errorf("expected version=2.0, got %v", entry["version"])
	}
}

func TestLogger_Print_GroupAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Print(
		slog.Group("info",
			slog.String("name", "aenv"),
			slog.Int("pid", 42),
		),
	)

	entry := parseJSON(t, buf.String())

	group, ok := entry["info"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'info' as object, got %T", entry["info"])
	}
	if group["name"] != "aenv" {
		t.Errorf("expected info.name=aenv, got %v", group["name"])
	}
}

func TestLogger_Print_Text_GroupAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatText),
		WithPretty(false),
	)

	logger.Print(
		slog.Group("info",
			slog.String("name", "aenv"),
			slog.Int("pid", 42),
		),
	)

	output := buf.String()

	// Group should be rendered in text format
	if !strings.Contains(output, "info=") {
		t.Errorf("expected info= in text output, got: %s", output)
	}
	if !strings.Contains(output, "name=aenv") {
		t.Errorf("expected name=aenv in text group, got: %s", output)
	}
}

func TestLogger_Print_Text_EndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatText),
		WithPretty(false),
	)

	logger.Print(slog.String("k", "v"))

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("expected trailing newline, got: %q", output)
	}
}

func TestLogger_Print_JSON_EndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf, LevelInfo)

	logger.Print(slog.String("k", "v"))

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("expected trailing newline, got: %q", output)
	}
}

func TestLogger_Print_ConcurrentCalls_ThreadSafe(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf,
		WithFormat(FormatJSON),
		WithPretty(false),
	)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Print(slog.Int("id", id))
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 lines, got %d", len(lines))
	}
}

func TestLogger_Raw_WritesExactString(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	logger.Raw("hello world")

	if got := buf.String(); got != "hello world\n" {
		t.Errorf("expected %q, got %q", "hello world\n", got)
	}
}

func TestLogger_Raw_PreservesTrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	logger.Raw("already has newline\n")

	if got := buf.String(); got != "already has newline\n" {
		t.Errorf("expected %q, got %q", "already has newline\n", got)
	}
}

func TestLogger_Raw_EmptyString_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	logger.Raw("")

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty string, got %q", buf.String())
	}
}

func TestLogger_Raw_ZeroValue_NoOutput(t *testing.T) {
	var logger Logger
	logger.Raw("should not panic")
}

func TestLogger_Raw_IgnoresLogLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf, WithLevel(LevelError))

	logger.Raw("visible regardless of level")

	if buf.Len() == 0 {
		t.Error("expected output regardless of log level")
	}
}

func TestLogger_Raw_ConcurrentCalls_ThreadSafe(t *testing.T) {
	var buf bytes.Buffer
	logger := Make(&buf)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Raw("line")
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 lines, got %d", len(lines))
	}
}

// BenchmarkLogger_Callsite_Overhead benchmarks the cost of including callsite
// information by comparing with and without.
func BenchmarkLogger_Callsite_Overhead(b *testing.B) {
	b.Run("without_callsite", func(b *testing.B) {
		var buf bytes.Buffer
		logger := Make(&buf, WithCallsite(false), WithPretty(false))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("bench", slog.Int("i", i))
			buf.Reset()
		}
	})

	b.Run("with_callsite", func(b *testing.B) {
		var buf bytes.Buffer
		logger := Make(&buf, WithCallsite(true), WithPretty(false))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("bench", slog.Int("i", i))
			buf.Reset()
		}
	})
}
