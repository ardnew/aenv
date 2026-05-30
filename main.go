package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"

	"github.com/ardnew/aenv/cli"
	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/pkg"
)

// Version is the semantic version of the aenv module embedded at build time.
// It is printed by the CLI when users invoke the version subcommand.
//
//go:embed VERSION
var Version string

// License is the license of the aenv module embedded at build time.
// It is printed by the CLI when users invoke the version subcommand.
//
//go:embed LICENSE
var License string

func init() {
	pkg.Meta.Version = Version
	pkg.Meta.License = License
}

func main() {
	ctx := context.Background()
	if err := cli.Run(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		var c exit.Coder
		if errors.As(err, &c) {
			os.Exit(c.ExitCode())
		}
		os.Exit(exit.Software)
	}
}
