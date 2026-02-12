package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestWithSourceFilesEmpty tests that an empty source list returns nil reader.
func TestWithSourceFilesEmpty(t *testing.T) {
	ctx := WithSourceFiles(context.Background(), nil)
	reader := sourceFilesFrom(ctx)

	if reader != nil {
		t.Error("WithSourceFiles(nil) should store nil reader")
	}

	ctx = WithSourceFiles(context.Background(), []string{})
	reader = sourceFilesFrom(ctx)

	if reader != nil {
		t.Error("WithSourceFiles([]) should store nil reader")
	}
}

// TestWithSourceFilesSingleFile tests reading from a single file.
func TestWithSourceFilesSingleFile(t *testing.T) {
	// Create temp file
	tmpfile, err := os.CreateTemp("", "aenv-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "hello world"
	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	ctx := WithSourceFiles(context.Background(), []string{tmpfile.Name()})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader for valid file")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

// TestWithSourceFilesMultipleFiles tests reading from multiple files.
func TestWithSourceFilesMultipleFiles(t *testing.T) {
	// Create temp files
	tmpdir, err := os.MkdirTemp("", "aenv-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	file1 := filepath.Join(tmpdir, "file1.txt")
	file2 := filepath.Join(tmpdir, "file2.txt")

	if err := os.WriteFile(file1, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file2, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := WithSourceFiles(context.Background(), []string{file1, file2})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	if string(data) != "firstsecond" {
		t.Errorf("got %q, want %q", string(data), "firstsecond")
	}
}

// TestWithSourceFilesDuplicatePaths tests deduplication of identical paths.
func TestWithSourceFilesDuplicatePaths(t *testing.T) {
	// Create temp file
	tmpfile, err := os.CreateTemp("", "aenv-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "unique"
	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Pass same file multiple times
	ctx := WithSourceFiles(context.Background(), []string{
		tmpfile.Name(),
		tmpfile.Name(),
		tmpfile.Name(),
	})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	// Should only read once despite being listed 3 times
	if string(data) != content {
		t.Errorf("got %q, want %q (file should only be read once)", string(data), content)
	}
}

// TestWithSourceFilesRelativeAbsoluteDuplicates tests dedup of relative and
// absolute paths pointing to the same file.
func TestWithSourceFilesRelativeAbsoluteDuplicates(t *testing.T) {
	// Create temp file in current directory
	tmpdir, err := os.MkdirTemp("", "aenv-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	filename := "testfile.txt"
	absPath := filepath.Join(tmpdir, filename)
	content := "content"

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory to test relative paths
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpdir); err != nil {
		t.Fatal(err)
	}

	// Pass both relative and absolute paths
	ctx := WithSourceFiles(context.Background(), []string{
		filename, // relative
		absPath,  // absolute
	})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	// Should only read once
	if string(data) != content {
		t.Errorf("got %q, want %q (file should only be read once)", string(data), content)
	}
}

// TestWithSourceFilesSymlinkDuplicates tests dedup of symlinks pointing to the
// same file.
func TestWithSourceFilesSymlinkDuplicates(t *testing.T) {
	// Create temp directory
	tmpdir, err := os.MkdirTemp("", "aenv-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// Create actual file
	realFile := filepath.Join(tmpdir, "real.txt")
	content := "symlink-test"

	if err := os.WriteFile(realFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create symlink
	symlink := filepath.Join(tmpdir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Fatal(err)
	}

	// Pass both real file and symlink
	ctx := WithSourceFiles(context.Background(), []string{
		realFile,
		symlink,
	})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	// Should only read once
	if string(data) != content {
		t.Errorf("got %q, want %q (file should only be read once)", string(data), content)
	}
}

// TestWithSourceFilesStdinLast tests that stdin is placed last.
func TestWithSourceFilesStdinLast(t *testing.T) {
	// Create temp file
	tmpdir, err := os.MkdirTemp("", "aenv-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	file1 := filepath.Join(tmpdir, "file1.txt")
	if err := os.WriteFile(file1, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save and restore stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create pipe for stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write to stdin in goroutine
	go func() {
		defer w.Close()
		io.WriteString(w, "stdin")
	}()

	// Pass stdin first, then file - stdin should still be read last
	ctx := WithSourceFiles(context.Background(), []string{"-", file1})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	// File should be first, stdin last
	if string(data) != "filestdin" {
		t.Errorf("got %q, want %q (stdin should be last)", string(data), "filestdin")
	}
}

// TestWithSourceFilesMultipleStdinCollapsed tests that multiple "-" entries are
// collapsed to a single stdin reader.
func TestWithSourceFilesMultipleStdinCollapsed(t *testing.T) {
	// Save and restore stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create pipe for stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	content := "stdin-once"
	go func() {
		defer w.Close()
		io.WriteString(w, content)
	}()

	// Pass multiple stdin indicators
	ctx := WithSourceFiles(context.Background(), []string{"-", "-", "-"})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	// Should only read stdin once
	if string(data) != content {
		t.Errorf("got %q, want %q (stdin should only be read once)", string(data), content)
	}
}

// TestWithSourceFilesNonexistentFile tests that nonexistent files are skipped.
func TestWithSourceFilesNonexistentFile(t *testing.T) {
	// Create one real file
	tmpfile, err := os.CreateTemp("", "aenv-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "exists"
	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Pass mix of existing and nonexistent files
	ctx := WithSourceFiles(context.Background(), []string{
		"/nonexistent/path/file.txt",
		tmpfile.Name(),
		"/another/nonexistent.txt",
	})
	reader := sourceFilesFrom(ctx)

	if reader == nil {
		t.Fatal("WithSourceFiles should return non-nil reader when at least one file exists")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from source files: %v", err)
	}

	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

// TestWithSourceFilesAllNonexistent tests that all nonexistent files results in
// nil reader.
func TestWithSourceFilesAllNonexistent(t *testing.T) {
	ctx := WithSourceFiles(context.Background(), []string{
		"/nonexistent/path/file1.txt",
		"/nonexistent/path/file2.txt",
	})
	reader := sourceFilesFrom(ctx)

	if reader != nil {
		t.Error("WithSourceFiles should return nil reader when all files nonexistent")
	}
}
