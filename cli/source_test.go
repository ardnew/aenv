package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ardnew/aenv/log"
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

func TestSourcePaths_AppendsDiscoveredEntry(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)
	entry := filepath.Join(tmp, ".aenv")
	if err := os.WriteFile(entry, []byte("# entry"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	explicit := []string{"a.aenv", "b.aenv"}
	got := sourcePaths(explicit)
	if len(got) < len(explicit) {
		t.Fatalf("sourcePaths() = %v, want at least %v", got, explicit)
	}
	if !slices.Equal(got[:len(explicit)], explicit) {
		t.Fatalf("sourcePaths() prefix = %v, want %v", got[:len(explicit)], explicit)
	}
	if !slices.Contains(got, entry) {
		t.Fatalf("sourcePaths() = %v, want to contain discovered entry %q", got, entry)
	}
}

func TestLogSources_EmitsOnePerSource(t *testing.T) {
	restoreDefaultLogger(t)
	tmp := t.TempDir()
	chdir(t, tmp) // avoid discovering an ambient entry file

	var buf terminalWriter
	driver, err := log.New(log.HandlerOptions{Writer: &buf, Format: log.FormatText, Level: log.LevelTrace})
	if err != nil {
		t.Fatalf("log.New() error = %v", err)
	}
	log.SetDefault(driver)

	logSources([]string{"one.aenv", "two.aenv"})

	out := buf.String()
	if got := strings.Count(out, "processing source"); got < 2 {
		t.Fatalf("logSources emitted %d records, want at least 2:\n%s", got, out)
	}
	for _, want := range []string{"one.aenv", "two.aenv"} {
		if strings.Count(out, want) != 1 {
			t.Fatalf("logSources output want exactly one %q:\n%s", want, out)
		}
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
