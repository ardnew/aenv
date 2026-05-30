package cli

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"

	"github.com/alecthomas/kong"
)

type terminalWriter struct {
	bytes.Buffer
}

var errTestWrite = errors.New("write failed")

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errTestWrite
}

func (*terminalWriter) IsTerminalWriter() bool {
	return true
}

func newTestParser(t *testing.T, syntax *syntax, out io.Writer) *kong.Kong {
	t.Helper()
	parser, err := kong.New(
		syntax,
		kong.Name(pkg.Name),
		kong.Description(pkg.Description),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:        false,
			Summary:        true,
			WrapUpperBound: outputWidthMax,
		}),
		kong.Vars{"logHandlerSyntax": logHandlerSyntax},
		kong.Writers(out, out),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New() error = %v", err)
	}
	return parser
}

func TestLogHandlerSpec_UnmarshalText_Valid(t *testing.T) {
	jsonFormat := log.FormatJSON
	textFormat := log.FormatText

	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantFormat log.Format
		wantLevel  log.Level
	}{
		{name: "stdout", input: "-", wantOutput: "-", wantLevel: log.LevelInfo},
		{name: "default output json", input: ",json", wantFormat: jsonFormat, wantLevel: log.LevelInfo},
		{name: "default output debug", input: ",,debug", wantLevel: log.LevelDebug},
		{name: "file", input: "foo.log", wantOutput: "foo.log", wantLevel: log.LevelInfo},
		{name: "file empty trailing fields", input: "foo.log,,", wantOutput: "foo.log", wantLevel: log.LevelInfo},
		{name: "file text empty level", input: "foo.log,text,", wantOutput: "foo.log", wantFormat: textFormat, wantLevel: log.LevelInfo},
		{name: "stderr json trace", input: "stderr,json,trace", wantOutput: "stderr", wantFormat: jsonFormat, wantLevel: log.LevelTrace},
		{name: "empty output fields", input: ",,", wantLevel: log.LevelInfo},
		{name: "empty output format", input: ",", wantLevel: log.LevelInfo},
		{name: "empty", input: "", wantLevel: log.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got logHandlerSpec
			if err := got.UnmarshalText([]byte(tt.input)); err != nil {
				t.Fatalf("UnmarshalText() error = %v", err)
			}
			if got.output != tt.wantOutput {
				t.Fatalf("output = %q, want %q", got.output, tt.wantOutput)
			}
			if got.level != tt.wantLevel {
				t.Fatalf("level = %v, want %v", got.level, tt.wantLevel)
			}
			if got.format != tt.wantFormat {
				t.Fatalf("format = %v, want %v", got.format, tt.wantFormat)
			}
		})
	}
}

func TestLogHandlerSpec_UnmarshalText_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "level in format field", input: "-,info", wantErr: "invalid format"},
		{name: "format in level field", input: "-,,text", wantErr: "invalid level"},
		{name: "too many fields", input: "-,json,info,extra", wantErr: "expected output[,format[,level]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got logHandlerSpec
			err := got.UnmarshalText([]byte(tt.input))
			if err == nil {
				t.Fatal("UnmarshalText() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestSyntax_PositionalBranching(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "namespace", args: []string{"foo"}},
		{name: "namespace args", args: []string{"foo", "a", "b"}},
		{name: "version", args: []string{"version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var syntax syntax
			parser := newTestParser(t, &syntax, io.Discard)
			if _, err := parser.Parse(tt.args); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
		})
	}
}

func TestSyntax_LogFlagsScopedToNamespace(t *testing.T) {
	t.Run("namespace accepts empty log", func(t *testing.T) {
		var syntax syntax
		parser := newTestParser(t, &syntax, io.Discard)
		if _, err := parser.Parse([]string{"foo", "--log="}); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if len(syntax.Namespace.Log) != 1 {
			t.Fatalf("len(Log) = %d, want 1", len(syntax.Namespace.Log))
		}
		if syntax.Namespace.Log[0].output != "" || syntax.Namespace.Log[0].format.Valid() || syntax.Namespace.Log[0].level != log.LevelInfo {
			t.Fatalf("Log[0] = %#v, want empty output, derived format, info", syntax.Namespace.Log[0])
		}
	})

	t.Run("namespace accepts verbose", func(t *testing.T) {
		var syntax syntax
		parser := newTestParser(t, &syntax, io.Discard)
		if _, err := parser.Parse([]string{"foo", "-vv"}); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if syntax.Namespace.Verbose != 2 {
			t.Fatalf("Verbose = %d, want 2", syntax.Namespace.Verbose)
		}
	})

	tests := []struct {
		name string
		args []string
	}{
		{name: "root log", args: []string{"--log="}},
		{name: "version log", args: []string{"version", "--log="}},
		{name: "version verbose", args: []string{"version", "-v"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var syntax syntax
			parser := newTestParser(t, &syntax, io.Discard)
			if _, err := parser.Parse(tt.args); err == nil {
				t.Fatal("Parse() error = nil")
			}
		})
	}
}

func helpText(t *testing.T, args ...string) string {
	t.Helper()
	var syntax syntax
	var out bytes.Buffer
	parser := newTestParser(t, &syntax, &out)
	_, _ = parser.Parse(args)
	return out.String()
}

func TestHelp_RootShowsNamespaceBranchAndNoLogFlags(t *testing.T) {
	got := helpText(t, "--help")
	wants := []string{
		"Usage: aenv <command>",
		"<namespace> [<args> ...] [flags]",
		"version [flags]",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"--log=", "--verbose"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("help contains %q in %q", unwanted, got)
		}
	}
}

func TestHelp_NamespaceShowsNamespaceFlags(t *testing.T) {
	got := helpText(t, "foo", "--help")
	wants := []string{
		"Usage: aenv <namespace> [<args> ...] [flags]",
		"[<args> ...]",
		"--log=output[,format[,level]]",
		"--verbose",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q in %q", want, got)
		}
	}
}

func TestHelp_VersionShowsOnlyVersionFlags(t *testing.T) {
	got := helpText(t, "version", "--help")
	wants := []string{
		"Usage: aenv version [flags]",
		"--semantic",
		"--url",
		"--repo",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"--log=", "--verbose"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("help contains %q in %q", unwanted, got)
		}
	}
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var syntax syntax
	var out bytes.Buffer
	parser := newTestParser(t, &syntax, &out)
	ctx, err := parser.Parse(args)
	if err != nil {
		return out.String(), err
	}
	err = ctx.Run()
	return out.String(), err
}

func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	var coder exit.Coder
	if !errors.As(err, &coder) {
		t.Fatalf("errors.As(%T, exit.Coder) = false", err)
	}
	if got := coder.ExitCode(); got != want {
		t.Fatalf("ExitCode() = %d, want %d", got, want)
	}
}

func parseLogSpec(t *testing.T, value string) logHandlerSpec {
	t.Helper()
	var spec logHandlerSpec
	if err := spec.UnmarshalText([]byte(value)); err != nil {
		t.Fatalf("UnmarshalText(%q) error = %v", value, err)
	}
	return spec
}

func TestVersionCmd_Run_PrintsBrief(t *testing.T) {
	got, err := runCLI(t, "version")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{pkg.Name, strings.TrimSpace(pkg.Meta.Version)} {
		if !strings.Contains(got, want) {
			t.Fatalf("version output missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{`"level"`, `"message"`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("version output contains log field %q in %q", unwanted, got)
		}
	}
}

func TestVersionCmd_Run_SemanticPrintsPlainVersion(t *testing.T) {
	got, err := runCLI(t, "version", "--semantic")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if want := strings.TrimSpace(pkg.Meta.Version) + "\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestVersionCmd_Run_PrintsURLs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "project URL", args: []string{"version", "--url"}, want: pkg.ProjectURL + "\n"},
		{name: "repository URL", args: []string{"version", "--repo"}, want: pkg.RepoURL + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCLI(t, tt.args...)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionCmd_Run_RejectsMultipleSelectors(t *testing.T) {
	tests := [][]string{
		{"version", "--semantic", "--url"},
		{"version", "--semantic", "--repo"},
		{"version", "--url", "--repo"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := runCLI(t, args...); err == nil {
				t.Fatal("Run() error = nil")
			}
		})
	}
}

func TestVersionCmd_Run_RejectsSelectorValues(t *testing.T) {
	tests := [][]string{
		{"version", "--url=repo"},
		{"version", "--repo=project"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := runCLI(t, args...); err == nil {
				t.Fatal("Run() error = nil")
			}
		})
	}
}

func TestVersionCmd_Run_DoesNotLog(t *testing.T) {
	restoreDefaultLogger(t)
	var logs bytes.Buffer
	driver, err := log.New(log.HandlerOptions{Writer: &logs, Format: log.FormatJSON, Level: log.LevelTrace})
	if err != nil {
		t.Fatalf("log.New() error = %v", err)
	}
	log.SetDefault(driver)

	tests := [][]string{
		{"version"},
		{"version", "--semantic"},
		{"version", "--url"},
		{"version", "--repo"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			logs.Reset()
			if _, err := runCLI(t, args...); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := logs.String(); got != "" {
				t.Fatalf("version command logged %q", got)
			}
		})
	}
}

func TestVersionCmd_Run_ReturnsWriteError(t *testing.T) {
	var syntax syntax
	parser := newTestParser(t, &syntax, errorWriter{})
	ctx, err := parser.Parse([]string{"version"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err = ctx.Run()
	if !errors.Is(err, errTestWrite) {
		t.Fatalf("Run() error = %v, want %v", err, errTestWrite)
	}
	assertExitCode(t, err, exit.IO)
}

func TestEval_Run_ReturnsLogOutputCreateError(t *testing.T) {
	var syntax syntax
	parser := newTestParser(t, &syntax, io.Discard)
	ctx, err := parser.Parse([]string{"foo", "--log=" + t.TempDir()})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err = ctx.Run()
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("errors.As(%T, *fs.PathError) = false", err)
	}
	assertExitCode(t, err, exit.Create)
}

func TestAdjustLevel(t *testing.T) {
	tests := []struct {
		name    string
		base    log.Level
		verbose int
		want    log.Level
	}{
		{name: "unchanged", base: log.LevelWarn, want: log.LevelWarn},
		{name: "one step", base: log.LevelWarn, verbose: 1, want: log.LevelInfo},
		{name: "clamped", base: log.LevelInfo, verbose: 9, want: log.LevelTrace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adjustLevel(tt.base, tt.verbose); got != tt.want {
				t.Fatalf("adjustLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveFormat(t *testing.T) {
	var terminal terminalWriter
	var file bytes.Buffer
	jsonFormat := log.FormatJSON

	if got := resolveFormat(logHandlerSpec{}, &terminal); got != log.FormatText {
		t.Fatalf("resolveFormat(terminal) = %v, want %v", got, log.FormatText)
	}
	if got := resolveFormat(logHandlerSpec{}, &file); got != log.FormatJSON {
		t.Fatalf("resolveFormat(file) = %v, want %v", got, log.FormatJSON)
	}
	if got := resolveFormat(logHandlerSpec{format: jsonFormat}, &terminal); got != log.FormatJSON {
		t.Fatalf("resolveFormat(explicit) = %v, want %v", got, log.FormatJSON)
	}
}

func TestConfigureLogging_Default(t *testing.T) {
	restoreDefaultLogger(t)

	closers, err := openLogHandler(nil, 1)
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	cleanupClosers(t, closers)

	options := handlerOptions(t)
	if len(options) != 1 {
		t.Fatalf("len(handlers) = %d, want 1", len(options))
	}
	if options[0].Writer != os.Stdout || options[0].Format != log.FormatText || options[0].Level != log.LevelInfo {
		t.Fatalf("handler = %#v, want stdout text info", options[0])
	}
}

func TestConfigureLogging_FileSpecAddsDefault(t *testing.T) {
	restoreDefaultLogger(t)

	path := t.TempDir() + "/app.log"
	closers, err := openLogHandler([]logHandlerSpec{{output: path}}, 0)
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	cleanupClosers(t, closers)

	options := handlerOptions(t)
	if len(options) != 2 {
		t.Fatalf("len(handlers) = %d, want 2", len(options))
	}
	if options[0].Writer != os.Stdout || options[0].Format != log.FormatText || options[0].Level != log.LevelWarn {
		t.Fatalf("default handler = %#v, want stdout text warn", options[0])
	}
	file, ok := options[1].Writer.(*os.File)
	if !ok {
		t.Fatalf("file handler writer = %T, want *os.File", options[1].Writer)
	}
	if file.Name() != path || options[1].Format != log.FormatJSON || options[1].Level != log.LevelInfo {
		t.Fatalf("file handler = %#v, want %q json info", options[1], path)
	}
}

func TestConfigureLogging_ConsoleSpecReplacesDefault(t *testing.T) {
	restoreDefaultLogger(t)

	jsonFormat := log.FormatJSON
	closers, err := openLogHandler([]logHandlerSpec{{output: "stderr", format: jsonFormat, level: log.LevelTrace}}, 0)
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	cleanupClosers(t, closers)

	options := handlerOptions(t)
	if len(options) != 1 {
		t.Fatalf("len(handlers) = %d, want 1", len(options))
	}
	if options[0].Writer != os.Stderr || options[0].Format != log.FormatJSON || options[0].Level != log.LevelTrace {
		t.Fatalf("handler = %#v, want stderr json trace", options[0])
	}
}

func TestConfigureLogging_StdoutAliasReplacesDefault(t *testing.T) {
	restoreDefaultLogger(t)

	textFormat := log.FormatText
	closers, err := openLogHandler([]logHandlerSpec{{output: "stdout", format: textFormat}}, 0)
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	cleanupClosers(t, closers)

	options := handlerOptions(t)
	if len(options) != 1 {
		t.Fatalf("len(handlers) = %d, want 1", len(options))
	}
	if options[0].Writer != os.Stdout || options[0].Format != log.FormatText || options[0].Level != log.LevelInfo {
		t.Fatalf("handler = %#v, want stdout text info", options[0])
	}
}

func TestConfigureLogging_RepeatedOutputModifiesExistingHandler(t *testing.T) {
	restoreDefaultLogger(t)

	closers, err := openLogHandler([]logHandlerSpec{
		parseLogSpec(t, "stdout,text,debug"),
		parseLogSpec(t, "-,json,trace"),
	}, 0)
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	cleanupClosers(t, closers)

	options := handlerOptions(t)
	if len(options) != 1 {
		t.Fatalf("len(handlers) = %d, want 1", len(options))
	}
	if options[0].Writer != os.Stdout || options[0].Format != log.FormatJSON || options[0].Level != log.LevelTrace {
		t.Fatalf("handler = %#v, want stdout json trace", options[0])
	}
}

func TestConfigureLogging_RepeatedOutputKeepsOmittedFields(t *testing.T) {
	restoreDefaultLogger(t)

	closers, err := openLogHandler([]logHandlerSpec{
		parseLogSpec(t, "stdout,text,debug"),
		parseLogSpec(t, "-,,trace"),
	}, 0)
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	cleanupClosers(t, closers)

	options := handlerOptions(t)
	if len(options) != 1 {
		t.Fatalf("len(handlers) = %d, want 1", len(options))
	}
	if options[0].Writer != os.Stdout || options[0].Format != log.FormatText || options[0].Level != log.LevelTrace {
		t.Fatalf("handler = %#v, want stdout text trace", options[0])
	}
}

func restoreDefaultLogger(t *testing.T) {
	t.Helper()
	previous := log.Default()
	t.Cleanup(func() { log.SetDefault(previous) })
}

func handlerOptions(t *testing.T) []log.HandlerOptions {
	t.Helper()
	var options []log.HandlerOptions
	for handler := range log.Default().Handlers() {
		option, ok := handler.Options()
		if !ok {
			t.Fatal("handler.Options() ok = false")
		}
		options = append(options, option)
	}
	return options
}

func cleanupClosers(t *testing.T, closers []io.Closer) {
	t.Helper()
	t.Cleanup(func() {
		if err := closeLogHandlers(closers); err != nil {
			t.Errorf("closeAll() error = %v", err)
		}
	})
}
