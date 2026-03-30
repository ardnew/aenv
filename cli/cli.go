package cli

import (
	"context"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	mattnIsatty "github.com/mattn/go-isatty"

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
	Pretty  bool `help:"Enable colorized pretty printing"  short:"p"                                negatable:""`

	Source []string `help:"Include source file(s) ('-' for stdin)" name:"file" placeholder:"PATH" short:"f" type:"existingfile"`

	Init    cmd.Init    `cmd:"" help:"Initialize configuration file"`
	Fmt     cmd.Fmt     `cmd:"" help:"Format source files"`
	Eval    cmd.Eval    `cmd:"" help:"Evaluate expressions in context"  default:"withargs"`
	Env     cmd.Env     `cmd:"" help:"Generate namespaced environments"`
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

	// Default Pretty to true when stderr (the default log output) is a
	// terminal. This gives colorized output on interactive sessions and
	// plain output when piped or redirected.
	cli.Pretty = isatty(DefaultLogOutput)

	// Pre-scan for logger flags to ensure early configuration regardless of
	// flag position. TextUnmarshaler on logFormat/logLevel handles those flags
	// during normal parsing, but this early scan also catches boolean flags
	// like --pretty.
	cli.Log.scan(args, &cli.Pretty)

	// Save the resolved Pretty value. Kong's Parse will reset it to the zero
	// value (false) when the user doesn't pass --pretty/--no-pretty, which
	// would discard the TTY-based default computed above.
	prettyBeforeParse := cli.Pretty

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

	// Restore Pretty from the pre-parse value. Kong resets negatable bools
	// without a default tag to false when the flag is absent, which would
	// discard the TTY-based default and any explicit --pretty/--no-pretty
	// already resolved by scan.
	cli.Pretty = prettyBeforeParse

	// Stuff additional context values for use by commands
	ctx = cmd.WithContext(ctx, ktx)
	// Always include the aenv config file as the first source so its
	// environment is available to all subcommands.
	ctx = cmd.WithSourceFiles(
		ctx,
		append([]string{configFilePath}, cli.Source...),
	)
	ctx = cmd.WithHasUserSources(ctx, len(cli.Source) > 0)
	ctx = cmd.WithPretty(ctx, cli.Pretty)

	// Finalize logger configuration with all parsed values including
	// TimeLayout and Caller which don't use TextUnmarshaler.
	defer cli.Log.start(ctx, cli.Verbose, cli.Quiet, cli.Pretty)()

	// [pprofConfig.start] is no-op unless built with tag pprof and enabled.
	defer cli.Pprof.start(ctx)()

	// Execute the selected command
	return ktx.Run(ctx, &cli)
}

// isatty reports whether w appears to be connected to a terminal.
// It returns false if w is nil or is not an *os.File.
func isatty(w any) bool {
	f, ok := w.(*os.File)

	return ok && f != nil && mattnIsatty.IsTerminal(f.Fd())
}
