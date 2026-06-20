package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"
)

var exampleTime = time.Date(2026, time.May, 10, 12, 34, 56, 789000000, time.FixedZone("TEST", 2*60*60))

type exampleTerminalWriter struct {
	bytes.Buffer
}

func (*exampleTerminalWriter) IsTerminalWriter() bool {
	return true
}

func useExampleTime() func() {
	prev := timeNow
	timeNow = func() time.Time { return exampleTime }
	return func() {
		timeNow = prev
	}
}

func ExampleNew() {
	restoreTime := useExampleTime()
	defer restoreTime()

	var out exampleTerminalWriter
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		panic(err)
	}

	driver.Info([]slog.Attr{slog.String("scope", "cli")}, "starting", "up")
	fmt.Print(out.String())

	// Output:
	// 12:34:56.789   attr.scope=cli :: starting up
}

func ExampleDriver_Logf() {
	restoreTime := useExampleTime()
	defer restoreTime()

	var out exampleTerminalWriter
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelWarn})
	if err != nil {
		panic(err)
	}

	driver.Logf(LevelWarn, []slog.Attr{slog.Int("retry", 2)}, "attempt %d failed", 3)
	fmt.Print(out.String())

	// Output:
	// 12:34:56.789 - attr.retry=2 :: attempt 3 failed
}

func ExampleSetDefault() {
	restoreTime := useExampleTime()
	defer restoreTime()

	prev := Default()
	defer SetDefault(prev)

	var out exampleTerminalWriter
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		panic(err)
	}
	SetDefault(driver)

	Info([]slog.Attr{slog.String("component", "sync")}, "loaded", "config")
	fmt.Print(out.String())

	// Output:
	// 12:34:56.789   attr.component=sync :: loaded config
}

func ExampleDriver_AddHandlers() {
	restoreTime := useExampleTime()
	defer restoreTime()

	var textOut exampleTerminalWriter
	var jsonOut bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &textOut, Format: FormatText, Level: LevelInfo})
	if err != nil {
		panic(err)
	}
	err = driver.AddHandlers(HandlerOptions{Writer: &jsonOut, Format: FormatJSON, Level: LevelWarn})
	if err != nil {
		panic(err)
	}

	driver.Info([]slog.Attr{slog.String("region", "us-east-1")}, "booted")
	driver.Warn([]slog.Attr{slog.String("region", "us-east-1")}, "disk", "almost", "full")

	fmt.Println("text:")
	fmt.Print(textOut.String())
	fmt.Println("json:")
	fmt.Print(jsonOut.String())

	// Output:
	// text:
	// 12:34:56.789   attr.region=us-east-1 :: booted
	// 12:34:56.789 - attr.region=us-east-1 :: disk almost full
	// json:
	// {"time":"2026-05-10T12:34:56.789+0200","level":"warn","source":"log/example_test.go:99","scope":"log.ExampleDriver_AddHandlers","attr":{"region":"us-east-1"},"message":"disk almost full"}
}

func ExampleHandler_SetLevel() {
	restoreTime := useExampleTime()
	defer restoreTime()

	var out exampleTerminalWriter
	driver, err := New()
	if err != nil {
		panic(err)
	}
	err = driver.AddHandlers(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelWarn})
	if err != nil {
		panic(err)
	}

	driver.Info(nil, "hidden")

	err = driver.MapHandlers(IsEnabledHandler,
		func(h *Handler) error { return h.SetLevel(LevelInfo) },
	)
	if err != nil {
		panic(err)
	}

	driver.Info(nil, "visible", "now")
	fmt.Print(out.String())

	// Output:
	// 12:34:56.789   visible now
}
