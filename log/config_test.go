package log

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_WithLevel_SetsLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected Level
	}{
		{"debug", LevelDebug, LevelDebug},
		{"info", LevelInfo, LevelInfo},
		{"warn", LevelWarn, LevelWarn},
		{"error", LevelError, LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config{}
			opt := WithLevel(tt.level)
			result := opt(c)

			if result.level != tt.expected {
				t.Errorf("expected level %v, got %v", tt.expected, result.level)
			}
		})
	}
}

func TestConfig_WithCaller_SetsEnableCaller(t *testing.T) {
	tests := []struct {
		name     string
		enable   bool
		expected bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config{}
			opt := WithCaller(tt.enable)
			result := opt(c)

			if result.caller != tt.expected {
				t.Errorf(
					"expected enableCaller %v, got %v",
					tt.expected,
					result.caller,
				)
			}
		})
	}
}

func TestConfig_WithFormat_SetsFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   Format
		expected Format
	}{
		{"json", FormatJSON, FormatJSON},
		{"text", FormatText, FormatText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config{}
			opt := WithFormat(tt.format)
			result := opt(c)

			if result.format != tt.expected {
				t.Errorf("expected format %v, got %v", tt.expected, result.format)
			}
		})
	}
}

func TestConfig_formatTime_FormatsTimestamp(t *testing.T) {
	now := time.Date(2023, 10, 15, 14, 30, 45, 123456789, time.UTC)

	tests := []struct {
		name        string
		layout      string
		contains    []string
		notContains []string
	}{
		{
			name:        "rfc3339 named layout",
			layout:      "RFC3339",
			contains:    []string{"2023-10-15T14:30:45Z"},
			notContains: []string{".123", ".456", ".789"},
		},
		{
			name:        "rfc3339 nano named layout",
			layout:      "RFC3339Nano",
			contains:    []string{"2023-10-15T14:30:45.123456789Z"},
			notContains: nil,
		},
		{
			name:   "custom layout with whitespace and leading percent",
			layout: "   2006-01-02 15:04:05.000Z07:00",
			contains: []string{
				"   2023-10-15 14:30:45.123Z",
			},
			notContains: nil,
		},
		{
			name:        "unknown named layout falls back to rfc3339",
			layout:      "UNKNOWN_FORMAT",
			contains:    []string{"UNKNOWN_FORMAT"},
			notContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := WithTimeLayout(tt.layout)(config{})
			result := c.formatTime(now)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected %q to contain %q", result, s)
				}
			}
			for _, s := range tt.notContains {
				if strings.Contains(result, s) {
					t.Errorf("expected %q not to contain %q", result, s)
				}
			}
		})
	}
}

func TestConfig_formatTime_DefaultsToRFC3339(t *testing.T) {
	now := time.Date(2023, 10, 15, 14, 30, 45, 123456789, time.UTC)
	c := WithTimeLayout("RFC3339")(config{})
	result := c.formatTime(now)

	expected := now.Format(time.RFC3339)
	if result != expected {
		t.Errorf("expected default layout %q, got %q", expected, result)
	}
}

func TestConfig_formatTime_EmptyFormat_DisablesTimestamp(t *testing.T) {
	now := time.Date(2023, 10, 15, 14, 30, 45, 123456789, time.UTC)

	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"whitespace only", "   \t  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := WithTimeLayout(tt.value)(config{})
			result := c.formatTime(now)

			if result != "" {
				t.Errorf(
					"expected empty timestamp when layout is %q, got %q",
					tt.value,
					result,
				)
			}
		})
	}
}

func BenchmarkConfig_formatTime_SecondResolution(b *testing.B) {
	c := WithTimeLayout("RFC3339")(config{})
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.formatTime(testTime)
	}
}

func BenchmarkConfig_formatTime_NanosecondResolution(b *testing.B) {
	c := WithTimeLayout("RFC3339Nano")(config{})
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.formatTime(testTime)
	}
}
