package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/cli"
	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/pkg"
)

// Version is the module's semantic version, embedded at build time.
//
//go:embed VERSION
var Version string

// License is the module's license text, embedded at build time.
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
		if err, ok := err.(kong.ExitCoder); ok {
			os.Exit(err.ExitCode())
		}
		os.Exit(exit.Software)
	}
	os.Exit(0)
}
