package cmd

import (
	"bufio"
	"context"
	"log/slog"
	"os"

	"github.com/ardnew/aenv/lang"
)

// Fmt reads input from stdin, parses it, and formats it in the chosen format.
type Fmt struct {
	Native Native `cmd:"" default:"withargs" help:"Format as native aenv syntax (default)."`
	JSON   JSON   `cmd:""                    help:"Format as JSON."`
	YAML   YAML   `cmd:""                    help:"Format as YAML."`
	AST    AST    `cmd:""                    help:"Format as abstract syntax tree."`
}

// Native formats input as native aenv syntax.
type Native struct {
	Indent int `default:"2" help:"Indent width for formatted output" short:"i"`

	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the fmt command.
func (f *Native) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if f.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(f.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	ast, err := lang.ParseReader(bufio.NewReader(file))
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "native"))
	}

	return ast.Format(os.Stdout, f.Indent)
}

// JSON reads input from stdin, parses it, and outputs as JSON.
type JSON struct {
	Indent int `default:"2" help:"Indent width for JSON output" short:"i"`

	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the json command.
func (j *JSON) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if j.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(j.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	ast, err := lang.ParseReader(bufio.NewReader(file))
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "json"))
	}

	return ast.FormatJSON(os.Stdout, j.Indent)
}

// YAML reads input from stdin, parses it, and outputs as YAML.
type YAML struct {
	Indent int `default:"2" help:"Indent width for YAML output" short:"i"`

	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the yaml command.
func (y *YAML) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if y.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(y.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	ast, err := lang.ParseReader(bufio.NewReader(file))
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "yaml"))
	}

	return ast.FormatYAML(ctx, os.Stdout, y.Indent)
}

// AST formats input as an abstract syntax tree representation.
type AST struct {
	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the ast command.
func (a *AST) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if a.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(a.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	ast, err := lang.ParseReader(bufio.NewReader(file))
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "ast"))
	}

	// Print the AST to stdout
	ast.Print(os.Stdout)

	return nil
}
