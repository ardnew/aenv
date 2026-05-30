package log

import (
	"io"
	"log/slog"
	"testing"
)

func BenchmarkDriver_Disabled(b *testing.B) {
	driver, err := New(HandlerOptions{Writer: io.Discard, Format: FormatText, Level: LevelInfo})
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		driver.Trace([]slog.Attr{slog.Int("count", i)}, "hidden")
	}
}

func BenchmarkDriver_TextHandler(b *testing.B) {
	driver, err := New(HandlerOptions{Writer: io.Discard, Format: FormatText, Level: LevelTrace})
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		driver.Info([]slog.Attr{slog.Int("count", i)}, "hello", "world")
	}
}

func BenchmarkDriver_JSONHandler(b *testing.B) {
	driver, err := New(HandlerOptions{Writer: io.Discard, Format: FormatJSON, Level: LevelTrace})
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		driver.Info([]slog.Attr{slog.Int("count", i)}, "hello", "world")
	}
}

func BenchmarkDriver_Fanout(b *testing.B) {
	driver, err := New(
		HandlerOptions{Writer: io.Discard, Format: FormatText, Level: LevelTrace},
		HandlerOptions{Writer: io.Discard, Format: FormatJSON, Level: LevelTrace},
	)
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		driver.Info([]slog.Attr{slog.Int("count", i)}, "hello", "world")
	}
}
