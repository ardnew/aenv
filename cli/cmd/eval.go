package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ardnew/aenv/cli/cmd/repl"
	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

// Eval evaluates a namespace from a source file with the given arguments.
type Eval struct {
	Name string   `arg:"" help:"Namespace identifier to evaluate"          name:"name" optional:""`
	Args []string `arg:"" help:"Arguments to bind to namespace parameters" name:"args" optional:""`
}

// Run executes the eval command.
func (e *Eval) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)

	defer func(err *error) { cancel(*err) }(&err)

	ktx := kongContextFrom(ctx)

	cacheDir, ok := ktx.Model.Vars()[CacheIdentifier]
	if !ok {
		return ErrMissingCacheDir
	}

	logger := log.With(slog.String("cmd", "eval"))
	logger.TraceContext(
		ctx,
		"eval start",
		slog.String("name", e.Name),
		slog.Int("arg_count", len(e.Args)),
	)

	if e.Name == "" {
		return repl.Run(ctx, sourceFilesFrom(ctx), cacheDir, logger)
	}

	reader := sourceFilesFrom(ctx)
	if reader == nil {
		return ErrNoSource.With(slog.String("hint", "use -f PATH or - for stdin"))
	}

	ast, err := lang.ParseReader(
		ctx,
		reader,
		// lang.WithLogger(logger),
	)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("command", "eval"))
	}

	result, err := ast.EvaluateNamespace(ctx, e.Name, e.Args)
	if err != nil {
		return lang.WrapError(err).
			With(
				slog.String("command", "eval"),
				slog.String("namespace", e.Name),
			)
	}

	// Print result in native format
	fmt.Println(lang.FormatResult(result))

	return nil
}
