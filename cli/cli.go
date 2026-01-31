package cli

import (
	"context"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/cli/cmd"
	"github.com/ardnew/aenv/pkg"
)

// CLI is the top-level command-line interface for aenv.
type CLI struct {
	logConfig   `embed:"" group:"log"   prefix:"log-"`
	pprofConfig `embed:"" group:"pprof" prefix:"pprof-"`

	Init cmd.Init `cmd:"" help:"Initialize configuration file"`
	Fmt  cmd.Fmt  `cmd:"" help:"Format namespace definitions"`

	Eval cmd.Eval `cmd:"" default:"withargs" help:"Evaluate namespaces" hidden:""`
}

// Run executes the aenv CLI with the given context and arguments.
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

	err := mkdirAllRequired()
	if err != nil {
		return err
	}

	configFilePath := configPath(baseConfig)

	vars = vars.CloneWith(kong.Vars{
		cmd.ConfigNamespace: configFilePath,
	})

	vars = vars.CloneWith(cli.logConfig.vars())
	vars = vars.CloneWith(cli.pprofConfig.vars())

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
			[]kong.Group{cli.logConfig.group(), cli.pprofConfig.group()},
		),
		// kong.DefaultEnvars(pkg.Prefix()),
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
		kong.Configuration(kong.JSON, configFilePath+".json"),
		kong.Configuration(resolve(baseConfig), configFilePath),
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
	cli.logConfig.start(ctx)

	defer cli.pprofConfig.start(
		ctx,
	)() // no-op unless built with tag pprof and enabled

	// Execute the selected command
	return kongCtx.Run()
}
