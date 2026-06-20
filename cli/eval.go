package cli

import (
	"context"
	"io"
	"slices"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

// Eval is the eval subcommand.
type Eval struct {
	logFlags
	inputFlags

	ast lang.AST
}

// Run executes the eval subcommand.
func (e Eval) Run(ctx context.Context) error {
	e.Source = slices.DeleteFunc(e.Source,
		func(s string) bool { return s == "" })

	log.Debug(log.Attrs(
		"name", "eval",
		"sources", len(e.Source),
		"handlers", len(e.Log),
		"verbose", e.Verbose,
	), "command")
	return withLogHandlers(e.logFlags, func() error {
		if err := withSources(e.Source, &e); err != nil {
			return err
		}
		log.Debug(log.Attrs("cmd", "eval"))
		return withExitCode(repLoop(ctx, e.ast), exit.OS)
	})
}

func (e *Eval) ReadFrom(r io.Reader) (int64, error) {
	nb, err := e.ast.ReadFrom(r)
	if err != nil {
		log.Debug(log.Attrs("error", err), "command parse")
		return nb, withExitCode(err, exit.Data)
	}
	return nb, nil
}
