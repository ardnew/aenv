package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"
)

func TestLogOutputKey_NormalizesConsoleAndExpandsFiles(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "stdout"},
		{"-", "stdout"},
		{"stdout", "stdout"},
		{"stderr", "stderr"},
	}
	for _, tt := range tests {
		if got := logOutputKey(tt.in); got != tt.want {
			t.Fatalf("logOutputKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := logOutputKey("relative.log"); !filepath.IsAbs(got) {
		t.Fatalf("logOutputKey(file) = %q, want an absolute expanded path", got)
	}
}

type envReader struct {
	got []string
}

func (e *envReader) Write(p []byte) (int, error) {
	n, err := e.ReadFrom(bytes.NewReader(p))
	return int(n), err
}

func (e *envReader) ReadFrom(r io.Reader) (int64, error) {
	read, err := io.ReadAll(r)
	nb := int64(len(read))
	if err != nil {
		return nb, err
	}
	if strings.TrimSpace(string(read)) == "" {
		return nb, errors.New("empty source file")
	}
	e.got = append(e.got, string(read))
	return nb, nil
}

func TestSourcePaths_AppendsDiscoveredEntry(t *testing.T) {
	touch := func(path ...string) {
		for _, p := range path {
			if err := os.WriteFile(p, []byte(filepath.Base(p)), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", p, err)
			}
		}
	}
	bases := func(paths ...string) []string {
		r := make([]string, len(paths))
		for i := range paths {
			r[i] = filepath.Base(paths[i])
		}
		return r
	}

	cwd := t.TempDir()
	chdir(t, cwd)

	explicit := []string{
		filepath.Join(cwd, "a.aenv"),
		filepath.Join(cwd, "b.aenv"),
	}
	automatic := filepath.Join(cwd, "."+pkg.Name+"rc")

	touch(append(explicit, automatic)...)

	e := &envReader{}

	if err := withSources(explicit, e); err != nil {
		t.Fatal("default file sourced when explicit source files provided")
	}

	if !slices.Equal(e.got, bases(explicit...)) {
		t.Fatalf("withSources() = %v, want %v", e.got, bases(explicit...))
	}

	e.got = []string{}
	if err := withSources(nil, e); err != nil {
		t.Fatalf("withSources() error = %v", err)
	}

	if !slices.Equal(e.got, bases(automatic)) {
		t.Fatalf("withSources() = %v, want %v", e.got, bases(automatic))
	}
}

type discardReader struct{}

func (d discardReader) Write(p []byte) (int, error) {
	return io.Discard.Write(p)
}

func (d discardReader) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(io.Discard, r)
}

func TestWithSources_LogsOnePerSource(t *testing.T) {
	restoreDefaultLogger(t)
	tmp := t.TempDir()
	chdir(t, tmp) // avoid discovering an ambient entry file

	for _, src := range []string{"one.aenv", "two.aenv"} {
		if err := os.WriteFile(src, []byte(""), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", src, err)
		}
	}

	var buf terminalWriter
	driver, err := log.New(log.HandlerOptions{Writer: &buf, Format: log.FormatText, Level: log.LevelTrace})
	if err != nil {
		t.Fatalf("log.New() error = %v", err)
	}
	log.SetDefault(driver)

	if err := withSources([]string{"one.aenv", "two.aenv"}, discardReader{}); err != nil {
		t.Fatalf("withSources() error = %v", err)
	}

	out := buf.String()
	out = strings.ReplaceAll(out, "\r\n", "\n")
	if got := strings.Count(out, ":: read source\n"); got != 2 {
		t.Fatalf("withSources emitted %d read-source records, want 2:\n%s", got, out)
	}
	for _, want := range []string{"one.aenv", "two.aenv"} {
		if strings.Count(out, "attr.path="+want) != 1 {
			t.Fatalf("withSources output want exactly one %q attr:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "attr.kind=explicit"); got != 2 {
		t.Fatalf("withSources emitted %d explicit source attrs, want 2:\n%s", got, out)
	}
	if !strings.Contains(out, "attr.index=1") || !strings.Contains(out, "attr.index=2") {
		t.Fatalf("withSources output missing explicit source indices:\n%s", out)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatalf("Chdir(%q) restore error = %v", prev, err)
		}
	})
}
