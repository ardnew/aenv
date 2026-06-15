package cli

import (
	"context"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/cli/cmd"
	"github.com/ardnew/aenv/pkg"
)

// CLI is the top-level command-line interface for aenv,
// containing global flags and subcommands.
type CLI struct {
	Var kong.VersionFlag `hidden:""`

	Log   logConfig   `embed:"" group:"log"   prefix:"log-"`
	Pprof pprofConfig `embed:"" group:"pprof" prefix:"pprof-"`

	Verbose int  `help:"Increment log verbosity"           short:"v" type:"counter"`
	Quiet   bool `help:"Suppress all output except errors" short:"q"                default:"false"`

	Source []string `help:"Include source file(s) ('-' for stdin)" name:"file" placeholder:"PATH" short:"f" type:"existingfile"`

	Init    cmd.Init    `cmd:"" help:"Initialize configuration file"`
	Fmt     cmd.Fmt     `cmd:"" help:"Format source files"`
	Eval    cmd.Eval    `cmd:"" help:"Evaluate configuration and print results"           default:"withargs"`
	Env     cmd.Env     `cmd:"" help:"Evaluate namespace and print environment variables"`
	Version cmd.Version `cmd:"" help:"Print version information"`
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
		cmd.ConfigIdentifier:  configFilePath,
		cmd.CacheIdentifier:   cacheDir(),
		cmd.VersionIdentifier: strings.TrimSpace(pkg.Version),
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
				Tree:                false,
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
	// Always include the aenv config file as the first source so its
	// environment is available to all subcommands.
	ctx = cmd.WithSourceFiles(
		ctx,
		append([]string{configFilePath}, cli.Source...),
	)
	ctx = cmd.WithHasUserSources(ctx, len(cli.Source) > 0)

	// Finalize logger configuration with all parsed values including
	// TimeLayout and Caller which don't use TextUnmarshaler.
	defer cli.Log.start(ctx, cli.Verbose, cli.Quiet)()

	// [pprofConfig.start] is no-op unless built with tag pprof and enabled.
	defer cli.Pprof.start(ctx)()

	// Execute the selected command
	return ktx.Run(ctx, &cli)
}
