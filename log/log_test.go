package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
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
