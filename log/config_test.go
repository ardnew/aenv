package log

import (
	"bytes"
	"io"
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

func TestConfig_WithCallsite_SetsEnableCallsite(t *testing.T) {
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
			opt := WithCallsite(tt.enable)
			result := opt(c)

			if result.callsite != tt.expected {
				t.Errorf(
					"expected enableCallsite %v, got %v",
					tt.expected,
					result.callsite,
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

func TestConfig_WithOutput_EmptyInput_LeavesUnchanged(t *testing.T) {
	original := &bytes.Buffer{}
	c := config{output: original}

	result := WithOutput()(c)

	if result.output != original {
		t.Errorf("expected output to remain unchanged")
	}
}

func TestConfig_WithOutput_SingleNilWriter_UsesDiscard(t *testing.T) {
	c := config{}

	result := WithOutput(nil)(c)

	if result.output != io.Discard {
		t.Errorf("expected io.Discard for single nil writer, got %v", result.output)
	}
}

func TestConfig_WithOutput_SingleWriter_SetsDirectly(t *testing.T) {
	buf := &bytes.Buffer{}
	c := config{}

	result := WithOutput(buf)(c)

	if result.output != buf {
		t.Errorf("expected output to be set to buffer directly")
	}
}

func TestConfig_WithOutput_MultipleUniqueWriters_UsesMultiWriter(t *testing.T) {
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	c := config{}

	result := WithOutput(buf1, buf2)(c)

	// Write to the output and verify both buffers receive it
	_, _ = result.output.Write([]byte("test"))

	if buf1.String() != "test" || buf2.String() != "test" {
		t.Errorf("expected both buffers to receive write, got %q and %q",
			buf1.String(), buf2.String())
	}
}

func TestConfig_WithOutput_DuplicateWriters_Deduplicates(t *testing.T) {
	buf := &bytes.Buffer{}
	c := config{}

	result := WithOutput(buf, buf, buf)(c)

	// Should deduplicate to single writer
	_, _ = result.output.Write([]byte("x"))

	// If not deduplicated, would write 3 times resulting in "xxx"
	if buf.String() != "x" {
		t.Errorf("expected single write due to deduplication, got %q", buf.String())
	}
}

func TestConfig_WithOutput_MultipleWritersWithNils_RemovesNils(t *testing.T) {
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	c := config{}

	result := WithOutput(buf1, nil, buf2, nil)(c)

	// Write to the output and verify only non-nil buffers receive it
	_, _ = result.output.Write([]byte("test"))

	if buf1.String() != "test" || buf2.String() != "test" {
		t.Errorf("expected both non-nil buffers to receive write, got %q and %q",
			buf1.String(), buf2.String())
	}
}

func TestConfig_WithOutput_AllNilWriters_UsesDiscard(t *testing.T) {
	c := config{}

	result := WithOutput(nil, nil, nil)(c)

	// Multiple nils should deduplicate to single nil, then use io.Discard
	if result.output != io.Discard {
		t.Errorf("expected io.Discard for all nil writers, got %v", result.output)
	}
}

func TestConfig_WithOutput_DuplicatesWithNil_DeduplicatesAndRemovesNil(t *testing.T) {
	buf := &bytes.Buffer{}
	c := config{}

	result := WithOutput(buf, nil, buf, nil, buf)(c)

	// Should deduplicate to just buf (single writer)
	_, _ = result.output.Write([]byte("x"))

	if buf.String() != "x" {
		t.Errorf("expected single write due to deduplication, got %q", buf.String())
	}
}
