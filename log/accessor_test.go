package log

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T, w io.Writer) *Handler {
	t.Helper()
	h, err := newHandler(HandlerOptions{Writer: w, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("newHandler() error = %v", err)
	}
	return h
}

func TestHandler_Accessors_ReportConfiguredValues(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(t, &buf)

	if !h.Enabled() {
		t.Fatal("Enabled() = false, want true for a fresh handler")
	}
	opts, ok := h.Options()
	if !ok || opts.Writer != &buf || opts.Format != FormatText || opts.Level != LevelInfo {
		t.Fatalf("Options() = %+v, %v; want writer/text/info", opts, ok)
	}
	if w, ok := h.Writer(); !ok || w != &buf {
		t.Fatalf("Writer() = %v, %v; want &buf, true", w, ok)
	}
	if f, ok := h.Format(); !ok || f != FormatText {
		t.Fatalf("Format() = %v, %v; want text, true", f, ok)
	}
	if l, ok := h.Level(); !ok || l != LevelInfo {
		t.Fatalf("Level() = %v, %v; want info, true", l, ok)
	}
}

func TestHandler_Accessors_NilHandlerReportsAbsent(t *testing.T) {
	var h *Handler
	if h.Enabled() {
		t.Fatal("Enabled() = true, want false for nil handler")
	}
	if _, ok := h.Options(); ok {
		t.Fatal("Options() ok = true, want false")
	}
	if _, ok := h.Writer(); ok {
		t.Fatal("Writer() ok = true, want false")
	}
	if _, ok := h.Format(); ok {
		t.Fatal("Format() ok = true, want false")
	}
	if _, ok := h.Level(); ok {
		t.Fatal("Level() ok = true, want false")
	}
}

func TestHandler_DisableEnable_TogglesDelivery(t *testing.T) {
	var buf bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &buf, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = driver.AddHandlers(HandlerOptions{Writer: io.Discard, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("AddHandlers() error = %v", err)
	}
	for h := range driver.Handlers() {
		if err := h.Disable(); err != nil {
			t.Fatalf("Disable() error = %v", err)
		}
		if h.Enabled() {
			t.Fatal("Enabled() = true after Disable()")
		}
		if err := h.Enable(); err != nil {
			t.Fatalf("Enable() error = %v", err)
		}
		if !h.Enabled() {
			t.Fatal("Enabled() = false after Enable()")
		}
	}
}

func TestPackageWrappers_RouteThroughDefaultDriver(t *testing.T) {
	setTestNow(t)
	var buf bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &terminalBuffer{}, Format: FormatText, Level: LevelTrace})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	// Replace the sole handler's writer so we can inspect output.
	for h := range driver.Handlers() {
		if err := h.SetWriter(&buf); err != nil {
			t.Fatalf("SetWriter() error = %v", err)
		}
	}
	prev := Default()
	SetDefault(driver)
	t.Cleanup(func() { SetDefault(prev) })

	attr := []slog.Attr{slog.String("k", "v")}
	calls := []struct {
		name string
		emit func()
	}{
		{"Error", func() { Error(attr, "boom") }},
		{"Errorf", func() { Errorf(attr, "boom %d", 1) }},
		{"Warn", func() { Warn(attr, "warn") }},
		{"Warnf", func() { Warnf(attr, "warn %d", 2) }},
		{"Info", func() { Info(attr, "info") }},
		{"Infof", func() { Infof(attr, "info %d", 3) }},
		{"Debug", func() { Debug(attr, "debug") }},
		{"Debugf", func() { Debugf(attr, "debug %d", 4) }},
		{"Trace", func() { Trace(attr, "trace") }},
		{"Tracef", func() { Tracef(attr, "trace %d", 5) }},
		{"Log", func() { Log(LevelInfo, attr, "log") }},
		{"Logf", func() { Logf(LevelInfo, attr, "log %d", 6) }},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			buf.Reset()
			c.emit()
			if got := buf.String(); !strings.Contains(got, "k=v") {
				t.Fatalf("%s output = %q, want to contain attr k=v", c.name, got)
			}
		})
	}
}

func TestDriverMethods_EmitAtDeclaredLevels(t *testing.T) {
	setTestNow(t)
	var buf bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &buf, Format: FormatJSON, Level: LevelTrace})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	attr := []slog.Attr{slog.String("k", "v")}
	tests := []struct {
		name  string
		emit  func()
		level string
	}{
		{"Errorf", func() { driver.Errorf(attr, "x %d", 1) }, "error"},
		{"Warn", func() { driver.Warn(attr, "x") }, "warn"},
		{"Warnf", func() { driver.Warnf(attr, "x %d", 1) }, "warn"},
		{"Infof", func() { driver.Infof(attr, "x %d", 1) }, "info"},
		{"Debug", func() { driver.Debug(attr, "x") }, "debug"},
		{"Debugf", func() { driver.Debugf(attr, "x %d", 1) }, "debug"},
		{"Trace", func() { driver.Trace(attr, "x") }, "trace"},
		{"Tracef", func() { driver.Tracef(attr, "x %d", 1) }, "trace"},
		{"Log", func() { driver.Log(LevelInfo, attr, "x") }, "info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.emit()
			if got := buf.String(); !strings.Contains(got, `"level":"`+tt.level+`"`) {
				t.Fatalf("%s output = %q, want level %q", tt.name, got, tt.level)
			}
		})
	}
}
