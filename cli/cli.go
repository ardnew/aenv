package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"

	"github.com/alecthomas/kong"
)

// Namespace defines a namespaced environment generator as an argument with
// optional flags. Any additional arguments are forwarded when generating a
// parametric namespace and are ignored otherwise.
// Flags are used to modify the behavior of the environment generator and are
// applicable to all namespaces.
type Namespace struct {
	logFlags
	inputFlags

	// Namespace is the name of the environment namespace to generate.
	Namespace string   `arg:""`
	// Args are forwarded to the namespace generator.
	Args      []string `arg:"" optional:"" name:"args" help:"Namespace arguments."`
}

// Eval defines the eval subcommand and flags for starting an interactive REPL
// to evaluate namespaces and other expressions.
type Eval struct {
	logFlags
	inputFlags
}

// Version defines the version subcommand and its flags.
// The flags are mutually exclusive and modify the subcommand's output.
type Version struct {
	// Semantic prints only the semantic version string.
	Semantic bool `help:"Print only the semantic version." short:"s" xor:"version-output"`
	// URL prints the project URL.
	URL      bool `help:"Print the project URL." xor:"version-output"`
	// Repo prints the repository URL.
	Repo     bool `help:"Print the repository URL." xor:"version-output"`
	// License prints the license text.
	License  bool `help:"Print the license." xor:"version-output"`
}

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

// Run parses command-line arguments and executes the appropriate subcommand.
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
		kong.BindTo(ctx, (*context.Context)(nil)),
	).Run()
}

func (n Namespace) Run() error {
	return withLogHandlers(n.logFlags, func() error {
		logSources(n.Source)
		return nil
	})
}

func (e Eval) Run(ctx context.Context) error {
	return withLogHandlers(e.logFlags, func() error {
		logSources(e.Source)
		return withExitCode(runREPL(ctx), exit.OS)
	})
}

func (v Version) Run(app *kong.Kong) error {
	version := strings.TrimSpace(pkg.Meta.Version)
	var err error
	switch {
	case v.Semantic:
		_, err = fmt.Fprintln(app.Stdout, version)
	case v.URL:
		_, err = fmt.Fprintln(app.Stdout, pkg.ProjectURL)
	case v.Repo:
		_, err = fmt.Fprintln(app.Stdout, pkg.RepoURL)
	case v.License:
		_, err = fmt.Fprintln(app.Stdout, pkg.Meta.License)
	default:
		_, err = fmt.Fprintln(app.Stdout, pkg.Name, "version", version)
	}
	return withExitCode(err, exit.IO)
}

func wrapLogHandlerError(err error) error {
	if err, ok := errors.AsType[*fs.PathError](err); ok {
		return withExitCode(err, exit.Create)
	}
	return withExitCode(err, exit.Software)
}

func withLogHandlers(flags logFlags, fn func() error) (err error) {
	closers, err := openLogHandler(flags.Log, flags.Verbose)
	if err != nil {
		return wrapLogHandlerError(err)
	}

	defer func() { err = errors.Join(err, withExitCode(closeLogHandlers(closers), exit.IO)) }()

	return fn()
}

func logSources(source []string) {
	for _, str := range sourcePaths(source) {
		log.Info([]slog.Attr{slog.String("source", str)}, "processing source")
	}
}

func sourcePaths(source []string) []string {
	paths := append([]string(nil), source...)
	if path, ok := pkg.EntryPath(); ok {
		paths = append(paths, path)
	}
	return paths
}
