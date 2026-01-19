package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"

	"github.com/ardnew/envcomp/cmd/envcomp/cli/cmd"
	"github.com/ardnew/envcomp/pkg"
)

const (
	// DefaultDirMode is the default file mode for creating directories.
	DefaultDirMode  = 0o700
	configNamespace = "config"
)

// CLI is the top-level command-line interface for envcomp.
type CLI struct {
	log   `embed:"" group:"log"   prefix:"log-"`
	pprof `embed:"" group:"pprof" prefix:"pprof-"`

	Fmt cmd.Fmt `cmd:"" help:"Format namespace definitions."`
}

// Run executes the envcomp CLI with the given context and arguments.
// The exit function is called with the appropriate exit code upon completion.
func Run(
	ctx context.Context,
	exit func(code int),
	args ...string,
) error {
	var (
		cli  CLI
		vars kong.Vars
	)

	conf, err := initRuntime()
	if err != nil {
		return err
	}

	vars = vars.CloneWith(cli.log.vars())
	vars = vars.CloneWith(cli.pprof.vars())

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Pre-scan for logger flags to ensure early configuration regardless of
	// flag position. TextUnmarshaler on logFormat/logLevel handles those flags
	// during normal parsing, but this early scan also catches boolean flags
	// like --log-pretty.
	cli.scan(args)

	// Parse command line
	parser, err := kong.New(&cli,
		kong.Name(pkg.Name),
		kong.Description(pkg.Description),
		kong.UsageOnError(),
		kong.Exit(exit),
		kong.ExplicitGroups(
			[]kong.Group{cli.log.group(), cli.pprof.group()},
		),
		kong.DefaultEnvars(pkg.Prefix()),
		kong.BindSingletonProvider(func() context.Context {
			return ctx
		}),
		kong.ConfigureHelp(
			kong.HelpOptions{
				Compact:             true,
				Summary:             true,
				Tree:                false,
				FlagsLast:           false,
				NoAppSummary:        false,
				NoExpandSubcommands: true,
			}),
		kong.Configuration(kong.JSON, conf+".json"),
		kong.Configuration(loadNamespace(configNamespace), conf),
		vars,
	)
	if err != nil {
		return err
	}

	kongCtx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	// Finalize logger configuration with all parsed values including
	// TimeLayout and Caller which don't use TextUnmarshaler.
	cli.log.start(ctx)

	defer cli.pprof.start(
		ctx,
	)() // no-op unless built with tag pprof and enabled

	// Execute the selected command
	return kongCtx.Run()
}

func initRuntime() (conf string, err error) {
	err = os.MkdirAll(pkg.ConfigDir(), DefaultDirMode)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(pkg.CacheDir(), DefaultDirMode)
	if err != nil {
		return "", err
	}

	return filepath.Join(pkg.ConfigDir(), "config"), nil
}
