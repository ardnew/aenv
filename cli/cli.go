package cli

import (
	"context"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/cli/cmd"
	"github.com/ardnew/aenv/pkg"
)

// CLI is the top-level command-line interface for aenv.
type CLI struct {
	Log   logConfig   `embed:"" group:"log"   prefix:"log-"`
	Pprof pprofConfig `embed:"" group:"pprof" prefix:"pprof-"`

	Source []string `help:"Input source file(s) or '-' for stdin" name:"source" short:"s" type:"existingfile"`

	Init cmd.Init `cmd:"" help:"Initialize configuration file"`
	Fmt  cmd.Fmt  `cmd:"" help:"Format namespaces"`

	Eval cmd.Eval `cmd:"" default:"withargs" help:"Evaluate namespaces"`
}

// Run executes the aenv CLI with the given context and arguments.
// The exit function is called with the appropriate exit code upon completion.
func Run(
	ctx context.Context,
	exit func(code int),
	args ...string,
) error {
	var cli CLI

	err := mkdirAllRequired()
	if err != nil {
		return err
	}

	configFilePath := configPath(baseConfig)

	vars := kong.Vars{
		cmd.ConfigIdentifier: configFilePath,
		cmd.CacheIdentifier:  cacheDir(),
	}.
		CloneWith(cli.Log.vars()).
		CloneWith(cli.Pprof.vars())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Pre-scan for logger flags to ensure early configuration regardless of
	// flag position. TextUnmarshaler on logFormat/logLevel handles those flags
	// during normal parsing, but this early scan also catches boolean flags
	// like --log-pretty.
	cli.Log.scan(args)

	// Parse command line
	parser, err := kong.New(&cli,
		kong.Name(pkg.Name),
		kong.Description(pkg.Description),
		kong.UsageOnError(),
		kong.Exit(exit),
		kong.ExplicitGroups(
			[]kong.Group{cli.Log.group(), cli.Pprof.group()},
		),
		// kong.DefaultEnvars(pkg.Prefix()),
		kong.BindSingletonProvider(func() context.Context {
			return ctx
		}),
		kong.ConfigureHelp(
			kong.HelpOptions{
				Compact:             true,
				Summary:             true,
				Tree:                true,
				FlagsLast:           false,
				NoAppSummary:        false,
				NoExpandSubcommands: true,
			}),
		kong.Configuration(kong.JSON, configFilePath+".json"),
		kong.Configuration(resolve(ctx, baseConfig), configFilePath),
		vars,
	)
	if err != nil {
		return err
	}

	ktx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	// Stuff additional context values for use by commands
	ctx = cmd.WithContext(ctx, ktx)
	ctx = cmd.WithSourceFiles(ctx, cli.Source)

	// Finalize logger configuration with all parsed values including
	// TimeLayout and Caller which don't use TextUnmarshaler.
	defer cli.Log.start(ctx)()

	// [pprofConfig.start] is no-op unless built with tag pprof and enabled.
	defer cli.Pprof.start(ctx)()

	// Execute the selected command
	return ktx.Run(ctx, &cli)
}
