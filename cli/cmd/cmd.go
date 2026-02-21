package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/alecthomas/kong"
)

// ContextKey is used to store a [kong.Context] value in [context.Context].
type contextKey struct{}

// WithContext returns a new context.Context containing the given kong.Context.
func WithContext(ctx context.Context, ktx *kong.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, ktx)
}

func kongContextFrom(ctx context.Context) *kong.Context {
	ktx, ok := ctx.Value(contextKey{}).(*kong.Context)
	if !ok || ktx == nil {
		return nil
	}

	return ktx
}

type (
	sourceFilesKey         struct{}
	explicitSourceFilesKey struct{}
	sourceFiles            struct {
		read     []io.Reader
		hasStdin bool
	}

	SourceFiles interface {
		IsZero() bool
		Stdin() io.Reader
		io.Reader
		io.WriterTo
	}
)

// IsZero reports whether there are no source files.
func (s *sourceFiles) IsZero() bool { return len(s.read) == 0 }

// Stdin returns os.Stdin if stdin was included as a source, or nil otherwise.
func (s *sourceFiles) Stdin() io.Reader {
	if s.hasStdin {
		return os.Stdin
	}

	return nil
}

// Read implements io.Reader by reading from all source files in order,
// including stdin if present.
func (s *sourceFiles) Read(p []byte) (n int, err error) {
	readers := s.read
	if s.hasStdin {
		readers = append(readers, os.Stdin)
	}

	return io.MultiReader(readers...).Read(p)
}

// WriteTo implements io.WriterTo by writing all source files to w in order,
// including stdin if present.
func (s *sourceFiles) WriteTo(w io.Writer) (n int64, err error) {
	readers := s.read
	if s.hasStdin {
		readers = append(readers, os.Stdin)
	}

	return io.Copy(w, io.MultiReader(readers...))
}

// fileKey uniquely identifies a file by its device and inode numbers.
// This handles deduplication across symlinks, absolute/relative paths, and
// special device files.
type fileKey struct {
	dev uint64
	ino uint64
}

// stdinSource is the special source indicator for reading from stdin.
const stdinSource = "-"

// WithSourceFiles returns a new context.Context containing an [io.Reader] that
// reads from the given source files.
//
// The function deduplicates readers by resolving symlinks and comparing device/
// inode pairs. All occurrences of "-" are replaced with a single stdin reader.
// The stdin reader is placed last so it reads after all regular files.
func WithSourceFiles(ctx context.Context, sources []string) context.Context {
	return context.WithValue(ctx, sourceFilesKey{}, buildSourceFiles(sources))
}

// WithExplicitSourceFiles returns a new context.Context containing an
// [io.Reader] for only the explicitly specified source files (excluding any
// implicitly included files such as the config file).
func WithExplicitSourceFiles(
	ctx context.Context,
	sources []string,
) context.Context {
	return context.WithValue(
		ctx,
		explicitSourceFilesKey{},
		buildSourceFiles(sources),
	)
}

// buildSourceFiles constructs a SourceFiles from the given source paths.
// It deduplicates readers by resolving symlinks and comparing device/inode
// pairs. All occurrences of "-" are replaced with a single stdin reader placed
// last so it reads after all regular files.
func buildSourceFiles(sources []string) SourceFiles {
	if len(sources) == 0 {
		return nil
	}

	var srcs sourceFiles

	srcs.read = make([]io.Reader, 0, len(sources))
	seen := make(map[fileKey]struct{})

	stdinInfo, _ := os.Stdin.Stat()
	stdinKey, _ := makeFileKey(stdinInfo)

	for _, src := range sources {
		if src == stdinSource {
			seen[stdinKey] = struct{}{}

			continue
		}

		reader, ok := openUniqueFile(src, seen)
		if !ok {
			continue
		}

		srcs.read = append(srcs.read, reader)
	}

	// Stdin may have been included via "-" or as a named file.
	// Both of which will be represented by stdinKey in seen.
	_, srcs.hasStdin = seen[stdinKey]
	delete(seen, stdinKey)

	// If no files were successfully opened and no stdin, return nil
	if len(srcs.read) == 0 && !srcs.hasStdin {
		return nil
	}

	return &srcs
}

// openUniqueFile opens the file at path if it hasn't been seen before.
// It resolves symlinks and uses device/inode to detect duplicates.
// Returns the opened file and true if successful, or nil and false if the file
// is a duplicate or cannot be opened.
func openUniqueFile(path string, seen map[fileKey]struct{}) (io.Reader, bool) {
	// Resolve to absolute path to handle relative path duplicates.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, false
	}

	// Resolve symlinks to their target.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, false
	}

	// Get file info to extract device and inode.
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, false
	}

	key, ok := makeFileKey(info)
	if !ok {
		return nil, false
	}

	if _, exists := seen[key]; exists {
		return nil, false
	}

	seen[key] = struct{}{}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, false
	}

	return file, true
}

// makeFileKey creates a fileKey from os.FileInfo.
// Returns false if the underlying Sys() data is not of type *syscall.Stat_t.
func makeFileKey(info os.FileInfo) (key fileKey, ok bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return key, false
	}

	return fileKey{dev: stat.Dev, ino: stat.Ino}, true
}

// sourceFilesFrom retrieves the io.Reader stored in ctx by WithSourceFiles.
// Returns nil if no reader was stored.
func sourceFilesFrom(ctx context.Context) SourceFiles {
	r, _ := ctx.Value(sourceFilesKey{}).(SourceFiles)

	return r
}

// explicitSourceFilesFrom retrieves the io.Reader stored in ctx by
// WithExplicitSourceFiles. Returns nil if no reader was stored.
func explicitSourceFilesFrom(ctx context.Context) SourceFiles {
	r, _ := ctx.Value(explicitSourceFilesKey{}).(SourceFiles)

	return r
}
