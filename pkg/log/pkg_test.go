package log

import (
	"bytes"
	"log/slog"
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
