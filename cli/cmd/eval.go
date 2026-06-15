package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/ardnew/aenv/cli/cmd/repl"
	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

// Eval evaluates expr-lang expressions. Each positional argument is treated as
// a single expression, equivalent to one input line at the interactive REPL.
// When no arguments are provided, the interactive REPL is launched.
type Eval struct {
	Expr []string `arg:"" help:"Expressions to evaluate (expr-lang syntax, one per argument)" name:"expr" optional:""`
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
		slog.Int("expr_count", len(e.Expr)),
	)

	if len(e.Expr) == 0 {
		return repl.Run(ctx, sourceFilesFrom(ctx), cacheDir, logger)
	}

	var reader io.Reader

	if sf := sourceFilesFrom(ctx); sf != nil {
		reader = sf
	} else {
		reader = strings.NewReader("")
	}

	ast, err := lang.ParseReader(ctx, reader)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("command", "eval"))
	}

	for _, expr := range e.Expr {
		result, err := ast.EvaluateExpr(ctx, expr)
		if err != nil {
			return lang.WrapError(err).
				With(
					slog.String("command", "eval"),
					slog.String("expr", expr),
				)
		}

		fmt.Println(lang.FormatResult(result))
	}

	return nil
}
