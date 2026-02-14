package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/ardnew/aenv/lang"
)

// Fmt reads input from stdin, parses it, and formats it in the chosen format.
type Fmt struct {
	Native Native `cmd:"" default:"withargs" help:"Format as native aenv syntax (default)"`
	JSON   JSON   `cmd:""                    help:"Format as JSON"`
	YAML   YAML   `cmd:""                    help:"Format as YAML"`
	AST    AST    `cmd:""                    help:"Format as abstract syntax tree"`
}

// Native formats input as native aenv syntax.
type Native struct {
	Indent int `default:"2" help:"Indent width for formatted output" short:"i"`
}

// Run executes the fmt command.
func (f *Native) Run(ctx context.Context) error {
	reader := sourceFilesFrom(ctx)
	if reader == nil {
		return NewError("require source files")
	}

	ast, err := lang.ParseReader(ctx, reader)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "native"))
	}

	return ast.Format(ctx, os.Stdout, f.Indent)
}

// JSON reads input from stdin, parses it, and outputs as JSON.
type JSON struct {
	Indent int `default:"2" help:"Indent width for JSON output" short:"i"`
}

// Run executes the json command.
func (j *JSON) Run(ctx context.Context) error {
	reader := sourceFilesFrom(ctx)
	if reader == nil {
		return NewError("require source files")
	}

	ast, err := lang.ParseReader(ctx, reader)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "json"))
	}

	return ast.FormatJSON(ctx, os.Stdout, j.Indent)
}

// YAML reads input from stdin, parses it, and outputs as YAML.
type YAML struct {
	Indent int `default:"2" help:"Indent width for YAML output" short:"i"`
}

// Run executes the yaml command.
func (y *YAML) Run(ctx context.Context) error {
	reader := sourceFilesFrom(ctx)
	if reader == nil {
		return NewError("require source files")
	}

	ast, err := lang.ParseReader(ctx, reader)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "yaml"))
	}

	return ast.FormatYAML(ctx, os.Stdout, y.Indent)
}

// AST formats input as an abstract syntax tree representation.
type AST struct{}

// Run executes the ast command.
func (a *AST) Run(ctx context.Context) error {
	reader := sourceFilesFrom(ctx)
	if reader == nil {
		return NewError("require source files")
	}

	ast, err := lang.ParseReader(ctx, reader)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "ast"))
	}

	// Print the AST to stdout
	ast.Print(ctx, os.Stdout)

	return nil
}
