package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ardnew/aenv/lang"
)

// Eval evaluates a definition from a source file with the given arguments.
type Eval struct {
	Name   string   `arg:"" help:"Definition identifier to evaluate"          name:"name"`
	Args   []string `arg:"" help:"Arguments to bind to definition parameters" name:"args" optional:""`
	Source string   `       help:"Source input file or '-' for stdin"                                 default:"-" short:"f"`
}

// Run executes the eval command.
func (e *Eval) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if e.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(e.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	// Parse with expression compilation enabled
	ast, err := lang.ParseReader(
		bufio.NewReader(file),
		lang.WithCompileExprs(true),
	)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("command", "eval"))
	}

	// Evaluate the definition
	result, err := ast.EvaluateDefinition(e.Name, e.Args)
	if err != nil {
		return lang.WrapError(err).
			With(
				slog.String("command", "eval"),
				slog.String("definition", e.Name),
			)
	}

	// Print result in native format
	fmt.Println(lang.FormatResult(result))

	return nil
}
