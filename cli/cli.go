package cli

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"

	"github.com/alecthomas/kong"
)

type logFlags struct {
	Verbose int              `help:"Increase log verbosity (repeatable)." short:"v" type:"counter"`
	Log     []logHandlerSpec `help:"Add or modify a log handler (repeatable)." placeholder:"${logHandlerSyntax}" sep:"none"`
}

type inputFlags struct {
	Source []string `name:"source" short:"f" help:"Source namespace definitions exclusively from file(s)." placeholder:"file" type:"existingfile" sep:","`
}

type syntax struct {
	Namespace Namespace `arg:"" help:"Generate environment variables from a parametric namespace."`

	Eval    Eval    `cmd:"" help:"Evaluate namespaces and other expressions in an interactive REPL."`
	Version Version `cmd:"" help:"Print version or related information."`
}

const outputWidthMax = 88 // you're gonna see some serious shit

// Run parses the command line and runs the selected subcommand.
func Run(ctx context.Context) error {
	var stx syntax

	return kong.Parse(
		&stx,
		kong.Name(pkg.Name),
		kong.Description(pkg.Description),
		kong.UsageOnError(),
		kong.ConfigureHelp(
			kong.HelpOptions{
				Compact:        true,
				Summary:        true,
				WrapUpperBound: outputWidthMax,
			},
		),
		kong.Vars{
			"logHandlerSyntax": logHandlerSyntax,
		},
		kong.BindTo(ctx, (*context.Context)(nil)), // bind the value, not a pointer
	).Run()
}

func wrapPathError(err error) error {
	if err, ok := errors.AsType[*fs.PathError](err); ok {
		return withExitCode(err, exit.Create)
	}
	return withExitCode(err, exit.Software)
}

func withLogHandlers(flags logFlags, fn func() error) (err error) {
	closers, err := openLogHandler(flags.Log, flags.Verbose)
	if err != nil {
		return wrapPathError(err)
	}

	defer func() {
		err = errors.Join(err, withExitCode(closeLogHandlers(closers), exit.IO))
	}()

	return fn()
}

type (
	sourceDef struct {
		path, kind   string
		index, count int
	}
	readerYielder func(io.ReaderFrom) (int64, error)
)

func makeExplicitSource(path string, idx, sum int) sourceDef {
	return sourceDef{path: path, kind: "explicit", index: idx, count: sum}
}

func makeDiscoveredSource(path string) sourceDef {
	return sourceDef{path: path, kind: "discovered"}
}

func (s sourceDef) attrs() []slog.Attr {
	attrs := log.Attrs("path", s.path, "kind", s.kind)
	if s.count > 1 { // show the index if there are multiple sources
		attrs = append(attrs, slog.Int("index", s.index))
	}
	return attrs
}

// yieldFrom is a helper that logs and opens a source file for reading
// by passing an [io.Reader] to the provided function.
//
// It guarantees that the file is closed after the function returns,
// so the caller can read as much as they want without cleaning up.
//
// It is called for each explicitly provided source, or, if no sources were
// provided, it is called once with the first discovered via [pkg.EntryPath].

func (s sourceDef) WriteTo(w io.ReaderFrom) (int64, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return 0, wrapPathError(err)
	}
	defer func() {
		closeErr := f.Close()
		if err == nil && closeErr != nil {
			err = withExitCode(closeErr, exit.IO)
		}
	}()
	log.Trace(s.attrs(), "read source")
	return w.ReadFrom(f)
}

func withSources(source []string, dst io.ReaderFrom) error {

	count := len(source)
	if count > 0 {
		log.Debug(log.Attrs("count", count), "explicit source(s) provided")
	}

	for i, src := range source {
		_, err := makeExplicitSource(src, i+1, count).WriteTo(dst)
		if err != nil {
			return err
		}
	}

	// Search for an entry file only if no explicit sources were provided.
	if count == 0 {
		str, ok := pkg.EntryPath()
		if !ok {
			return withExitCode(nil, exit.NoInput)
		}
		_, err := makeDiscoveredSource(str).WriteTo(dst)
		if err != nil {
			return err
		}
	}

	return nil
}
