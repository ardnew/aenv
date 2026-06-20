package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixedTestTime = time.Date(2026, time.May, 10, 12, 34, 56, 789000000, time.FixedZone("TEST", 2*60*60))

type terminalBuffer struct {
	bytes.Buffer
}

func (*terminalBuffer) IsTerminalWriter() bool {
	return true
}

type orderWriter struct {
	name  string
	order *[]string
	buf   *bytes.Buffer
}

func (w *orderWriter) Write(p []byte) (int, error) {
	*w.order = append(*w.order, w.name)
	return w.buf.Write(p)
}

type callSite struct {
	ref      string
	longPos  string
	shortPos string
}

func (site callSite) fileText() string {
	return "source=" + site.longPos + " scope=" + site.ref
}

func (site callSite) terminalText() string {
	return "source=" + site.shortPos + " scope=" + site.ref
}

func (site callSite) json() string {
	return fmt.Sprintf("\"source\":\"%s\",\"scope\":\"%s\"", site.longPos, site.ref)
}

func setTestNow(t *testing.T) {
	t.Helper()
	prev := timeNow
	timeNow = func() time.Time { return fixedTestTime }
	t.Cleanup(func() {
		timeNow = prev
	})
}

func nextCallSite(t *testing.T) callSite {
	t.Helper()
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return callSite{
		ref:      shortFuncName(runtime.FuncForPC(pc).Name()),
		longPos:  moduleRelativePath(file) + ":" + strconv.Itoa(line+1),
		shortPos: filepath.Base(file) + ":" + strconv.Itoa(line+1),
	}
}

func moduleRelativePath(path string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	rel, err := filepath.Rel(root, path)
	if err != nil {
		panic(err)
	}
	return filepath.ToSlash(rel)
}

func shortFuncName(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func TestDriver_Log_FileTextLayout(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info([]slog.Attr{slog.String("scope", "unit")}, "hello", "world")

	want := fmt.Sprintf(
		"time=%s level=info %s attr.scope=unit message=hello world\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("File text output mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_FileJSONLayout(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatJSON, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info([]slog.Attr{slog.Int("count", 7)}, "hello", "world")

	want := fmt.Sprintf(
		"{\"time\":\"%s\",\"level\":\"info\",%s,\"attr\":{\"count\":7},\"message\":\"hello world\"}\n",
		fixedTestTime.Format(longTimestampLayout),
		site.json(),
	)
	if got := out.String(); got != want {
		t.Fatalf("File JSON output mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_TerminalTextLayout(t *testing.T) {
	setTestNow(t)
	var out terminalBuffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelTrace})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Debug([]slog.Attr{slog.String("scope", "unit")}, "hello", "world")

	want := fmt.Sprintf(
		"%s · %s attr.scope=unit :: hello world\n",
		fixedTestTime.Format(shortTimestampLayout),
		site.terminalText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("Terminal text output mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_TerminalJSONLayout(t *testing.T) {
	setTestNow(t)
	var out terminalBuffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatJSON, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info([]slog.Attr{slog.Bool("ok", true)}, "hello", "world")

	want := fmt.Sprintf(
		"{\"time\":\"%s\",\"level\":\"info\",%s,\"attr\":{\"ok\":true},\"message\":\"hello world\"}\n",
		fixedTestTime.Format(shortTimestampLayout),
		site.json(),
	)
	if got := out.String(); got != want {
		t.Fatalf("Terminal JSON output mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_JoinedMessageParts(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info(nil, "hello", "from", "driver")

	want := fmt.Sprintf(
		"time=%s level=info %s message=hello from driver\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("Joined message mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_EmptyMessageOmitsSeparator(t *testing.T) {
	setTestNow(t)
	var out terminalBuffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	driver.Info(nil)

	want := fmt.Sprintf("%s  \n", fixedTestTime.Format(shortTimestampLayout))
	if got := out.String(); got != want {
		t.Fatalf("Empty message mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Logf_FormatErrors(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	format := "%d"
	site := nextCallSite(t)
	driver.Infof(nil, format, "nope")

	want := fmt.Sprintf(
		"time=%s level=info %s message=%%!d(string=nope)\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("Format error mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_MultiHandlerFanout(t *testing.T) {
	setTestNow(t)
	var textOut bytes.Buffer
	var jsonOut bytes.Buffer
	var termOut terminalBuffer
	driver, err := New(
		HandlerOptions{Writer: &textOut, Format: FormatText, Level: LevelInfo},
		HandlerOptions{Writer: &jsonOut, Format: FormatJSON, Level: LevelInfo},
		HandlerOptions{Writer: &termOut, Format: FormatText, Level: LevelInfo},
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info([]slog.Attr{slog.String("scope", "fanout")}, "hello")

	wantText := fmt.Sprintf(
		"time=%s level=info %s attr.scope=fanout message=hello\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	wantJSON := fmt.Sprintf(
		"{\"time\":\"%s\",\"level\":\"info\",%s,\"attr\":{\"scope\":\"fanout\"},\"message\":\"hello\"}\n",
		fixedTestTime.Format(longTimestampLayout),
		site.json(),
	)
	wantTerm := fmt.Sprintf("%s   attr.scope=fanout :: hello\n", fixedTestTime.Format(shortTimestampLayout))

	if got := textOut.String(); got != wantText {
		t.Fatalf("text handler mismatch\nwant: %q\ngot:  %q", wantText, got)
	}
	if got := jsonOut.String(); got != wantJSON {
		t.Fatalf("json handler mismatch\nwant: %q\ngot:  %q", wantJSON, got)
	}
	if got := termOut.String(); got != wantTerm {
		t.Fatalf("terminal handler mismatch\nwant: %q\ngot:  %q", wantTerm, got)
	}
}

func TestDefaultDriver_Info_PreservesCaller(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	prev := Default()
	SetDefault(driver)
	t.Cleanup(func() {
		SetDefault(prev)
	})

	site := nextCallSite(t)
	Info(nil, "wrapped")

	want := fmt.Sprintf(
		"time=%s level=info %s message=wrapped\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("wrapper caller mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_RemoveHandler_StopsDelivery(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = driver.AddHandlers(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("AddHandler() error = %v", err)
	}
	for h := range driver.Handlers() {
		if ok := driver.RemoveHandlers(h); !ok {
			t.Fatal("RemoveHandler() = false, want true")
		}
	}

	driver.Info(nil, "gone")

	if got := out.String(); got != "" {
		t.Fatalf("handler still received output: %q", got)
	}
}

func TestHandler_Setters_ReconfigureDelivery(t *testing.T) {
	setTestNow(t)
	var first bytes.Buffer
	var second bytes.Buffer
	driver, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = driver.AddHandlers(HandlerOptions{Writer: &first, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("AddHandler() error = %v", err)
	}

	var firstSite callSite

	for h := range driver.Handlers() {
		firstSite = nextCallSite(t)
		driver.Info(nil, "before")
		if err := h.Disable(); err != nil {
			t.Fatalf("Disable() error = %v", err)
		}
		driver.Error(nil, "hidden")
		if err := h.Enable(); err != nil {
			t.Fatalf("Enable() error = %v", err)
		}
		if err := h.SetLevel(LevelTrace); err != nil {
			t.Fatalf("SetLevel() error = %v", err)
		}
		if err := h.SetFormat(FormatJSON); err != nil {
			t.Fatalf("SetFormat() error = %v", err)
		}
		if err := h.SetWriter(&second); err != nil {
			t.Fatalf("SetWriter() error = %v", err)
		}
	}
	secondSite := nextCallSite(t)
	driver.Trace([]slog.Attr{slog.String("scope", "after")}, "after")

	wantFirst := fmt.Sprintf(
		"time=%s level=info %s message=before\n",
		fixedTestTime.Format(longTimestampLayout),
		firstSite.fileText(),
	)
	wantSecond := fmt.Sprintf(
		"{\"time\":\"%s\",\"level\":\"trace\",%s,\"attr\":{\"scope\":\"after\"},\"message\":\"after\"}\n",
		fixedTestTime.Format(longTimestampLayout),
		secondSite.json(),
	)
	if got := first.String(); got != wantFirst {
		t.Fatalf("first writer mismatch\nwant: %q\ngot:  %q", wantFirst, got)
	}
	if got := second.String(); got != wantSecond {
		t.Fatalf("second writer mismatch\nwant: %q\ngot:  %q", wantSecond, got)
	}
}

func TestDriver_Log_HandlerOrder(t *testing.T) {
	setTestNow(t)
	var order []string
	first := &orderWriter{name: "first", order: &order, buf: &bytes.Buffer{}}
	second := &orderWriter{name: "second", order: &order, buf: &bytes.Buffer{}}
	driver, err := New(
		HandlerOptions{Writer: first, Format: FormatText, Level: LevelInfo},
		HandlerOptions{Writer: second, Format: FormatText, Level: LevelInfo},
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	driver.Info(nil, "ordered")

	want := []string{"first", "second"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("handler order = %v, want %v", order, want)
	}
}

func TestDriver_Handlers_DoesNotOverwriteLaterUpdates(t *testing.T) {
	var first bytes.Buffer
	var second bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &first, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	handlers := driver.Handlers()
	if err := driver.AddHandlers(HandlerOptions{Writer: &second, Format: FormatText, Level: LevelInfo}); err != nil {
		t.Fatalf("AddHandlers() error = %v", err)
	}

	for range handlers {
	}

	var count int
	for range driver.Handlers() {
		count++
	}
	if count != 2 {
		t.Fatalf("handler count after stale iteration = %d, want 2", count)
	}
}

func TestLevel_String_SupportedValues(t *testing.T) {
	tests := []struct {
		name   string
		level  Level
		want   string
		symbol string
	}{
		{name: "error", level: LevelError, want: "error", symbol: "="},
		{name: "warn", level: LevelWarn, want: "warn", symbol: "-"},
		{name: "info", level: LevelInfo, want: "info", symbol: " "},
		{name: "debug", level: LevelDebug, want: "debug", symbol: "·"},
		{name: "trace", level: LevelTrace, want: "trace", symbol: ":"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.level.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
			if got := test.level.Symbol(); got != test.symbol {
				t.Fatalf("symbol() = %q, want %q", got, test.symbol)
			}
		})
	}
}

func TestDriver_Log_GroupAttrFlattening(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info([]slog.Attr{slog.Group("ctx", slog.Int("id", 7), slog.String("name", "svc"))}, "grouped")

	want := fmt.Sprintf(
		"time=%s level=info %s attr.ctx.id=7 attr.ctx.name=svc message=grouped\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("group attr output mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_UserAttrsStayUnderAttrNamespace(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatJSON, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info([]slog.Attr{
		slog.String("time", "user"),
		slog.String("message", "shadow"),
		slog.String("scope", "user-scope"),
		slog.Group("source", slog.String("pos", "user-pos"), slog.String("ref", "user-ref")),
	}, "hello")

	want := fmt.Sprintf(
		"{\"time\":\"%s\",\"level\":\"info\",%s,\"attr\":{\"time\":\"user\",\"message\":\"shadow\",\"scope\":\"user-scope\",\"source\":{\"pos\":\"user-pos\",\"ref\":\"user-ref\"}},\"message\":\"hello\"}\n",
		fixedTestTime.Format(longTimestampLayout),
		site.json(),
	)
	if got := out.String(); got != want {
		t.Fatalf("attr namespace mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_UnknownWriterUsesFileLayout(t *testing.T) {
	setTestNow(t)
	type unknownWriter struct {
		bytes.Buffer
	}
	var out unknownWriter
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info(nil, "file-layout")

	want := fmt.Sprintf(
		"time=%s level=info %s message=file-layout\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("fallback layout mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_LevelFiltering(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	driver.Debug(nil, "hidden")

	if got := out.String(); got != "" {
		t.Fatalf("filtered log produced output: %q", got)
	}
}

func TestDriver_Log_ZeroValueDriver(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	var driver Driver
	if err := driver.AddHandlers(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo}); err != nil {
		t.Fatalf("AddHandler() error = %v", err)
	}

	site := nextCallSite(t)
	driver.Info(nil, "zero")

	want := fmt.Sprintf(
		"time=%s level=info %s message=zero\n",
		fixedTestTime.Format(longTimestampLayout),
		site.fileText(),
	)
	if got := out.String(); got != want {
		t.Fatalf("zero-value driver output mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestDriver_Log_ConcurrentWrites(t *testing.T) {
	setTestNow(t)
	var out bytes.Buffer
	driver, err := New(HandlerOptions{Writer: &out, Format: FormatText, Level: LevelInfo})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const workers = 8
	const perWorker = 16
	var wait sync.WaitGroup
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < perWorker; iteration++ {
				driver.Info(nil, "entry", strconv.Itoa(worker), strconv.Itoa(iteration))
			}
		}(worker)
	}
	wait.Wait()

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	want := workers * perWorker
	if len(lines) != want {
		t.Fatalf("len(lines) = %d, want %d", len(lines), want)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "time=") {
			t.Fatalf("line missing prefix: %q", line)
		}
		if !strings.Contains(line, " source=") || !strings.Contains(line, " scope=") {
			t.Fatalf("line missing built-in source or scope: %q", line)
		}
		if !strings.Contains(line, " message=entry ") {
			t.Fatalf("line missing message payload: %q", line)
		}
	}
}
