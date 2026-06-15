package log

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"strings"
	"testing"
)

func TestPackage_LogFunctions_UseDefaultLogger(t *testing.T) {
	// Save original logger and restore after test
	original := defaultLog
	defer func() { defaultLog = original }()

	var buf bytes.Buffer
	// Configure default logger to write to buffer, use Debug level to capture all
	// logs
	defaultLog = Make(&buf, WithLevel(LevelDebug), WithFormat(FormatJSON), WithPretty(false))

	tests := []struct {
		name  string
		fn    func(string, ...slog.Attr)
		level string
		msg   string
	}{
		{"Debug", Debug, "DEBUG", "debug message"},
		{"Info", Info, "INFO", "info message"},
		{"Warn", Warn, "WARN", "warn message"},
		{"Error", Error, "ERROR", "error message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.fn(tt.msg, slog.String("key", "value"))

			output := buf.String()
			if !strings.Contains(output, tt.msg) {
				t.Errorf(
					"expected output to contain message %q, got: %s",
					tt.msg,
					output,
				)
			}
			if !strings.Contains(output, tt.level) {
				t.Errorf(
					"expected output to contain level %q, got: %s",
					tt.level,
					output,
				)
			}
			if !strings.Contains(output, `"key":"value"`) {
				t.Errorf("expected output to contain attribute, got: %s", output)
			}
		})
	}
}

func TestPackage_Callsite_ContextFunctions_ReportCaller(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	ctx := context.Background()

	original := defaultLog
	defer func() { defaultLog = original }()

	tests := []struct {
		name string
		fn   func(context.Context, string, ...slog.Attr)
	}{
		{"TraceContext", TraceContext},
		{"DebugContext", DebugContext},
		{"InfoContext", InfoContext},
		{"WarnContext", WarnContext},
		{"ErrorContext", ErrorContext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			defaultLog = Make(&buf,
				WithLevel(LevelTrace),
				WithCallsite(true),
				WithFormat(FormatJSON),
				WithPretty(false),
			)

			tt.fn(ctx, "caller test")
			src := parseSource(t, buf.String())

			if src.File != thisFile {
				t.Errorf("expected source file %q, got %q", thisFile, src.File)
			}
			if src.Function == "" {
				t.Error("expected non-empty source function")
			}
		})
	}
}

func TestPackage_Callsite_NonContextFunctions_ReportCaller(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)

	original := defaultLog
	defer func() { defaultLog = original }()

	tests := []struct {
		name string
		fn   func(string, ...slog.Attr)
	}{
		{"Trace", Trace},
		{"Debug", Debug},
		{"Info", Info},
		{"Warn", Warn},
		{"Error", Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			defaultLog = Make(&buf,
				WithLevel(LevelTrace),
				WithCallsite(true),
				WithFormat(FormatJSON),
				WithPretty(false),
			)

			tt.fn("caller test")
			src := parseSource(t, buf.String())

			if src.File != thisFile {
				t.Errorf("expected source file %q, got %q", thisFile, src.File)
			}
			if src.Function == "" {
				t.Error("expected non-empty source function")
			}
		})
	}
}
