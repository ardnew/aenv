package cmd

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

const (
	// ConfigNamespace is the kong variable identifier containing the name of
	// the default configuration namespace parsed from the configuration file.
	ConfigNamespace = "config"
	// DefaultDirMode is the default file mode for creating directories.
	// DefaultDirMode = 0o700.
)

var defaultConfigFormat = Native{Indent: 2, Source: ""}

// Init generates a default configuration file with current flag values.
type Init struct {
	Force bool `help:"Overwrite existing configuration file." short:"f"`
}

// Run executes the init command.
func (i *Init) Run(ctx *kong.Context) (err error) {
	conf, ok := ctx.Model.Vars()[ConfigNamespace]
	if !ok {
		panic("internal error: config namespace undefined")
	}

	// Check if file exists and force not set
	_, err = os.Stat(conf)
	if err == nil && !i.Force {
		return ErrWriteConfig.
			With(slog.String("file", conf)).
			With(slog.Bool("exists", true)).
			Wrap(errors.New("file already exists (use --force to overwrite)"))
	}

	// Open output file
	file, err := os.Create(conf)
	if err != nil {
		return ErrWriteConfig.
			With(slog.String("file", conf)).
			Wrap(err)
	}
	defer file.Close()

	var sb strings.Builder

	i.writeTo(&sb, ctx)

	p := lang.NewStream(strings.NewReader(sb.String()))

	ast, err := p.AST()
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "native"))
	}

	defaultConfigFormat.formatAST(ast, file)

	log.Debug("initialized configuration file", slog.String("path", conf))

	return nil
}

func (i *Init) writeTo(w io.Writer, ctx *kong.Context) {
	fmt.Fprintln(w, "config : {")

	// Extract values from the parsed CLI model
	i.writeFlag(w, ctx, "log-level")
	i.writeFlag(w, ctx, "log-format")
	i.writeFlag(w, ctx, "log-time-layout")
	i.writeFlag(w, ctx, "log-caller")
	i.writeFlag(w, ctx, "log-pretty")
	i.writeFlag(w, ctx, "pprof-mode")
	i.writeFlag(w, ctx, "pprof-dir")

	fmt.Fprintln(w, "}")
}

// writeFlag writes a flag value to the config file if it's set.
func (i *Init) writeFlag(w io.Writer, ctx *kong.Context, name string) {
	// Find the flag in the model
	idx := slices.IndexFunc(ctx.Model.Flags, func(flag *kong.Flag) bool {
		return flag.Name == name
	})
	if idx == -1 {
		// Flag not found
		return
	}

	flag := ctx.Model.Flags[idx]
	// Get the value from kong's internal scan
	val := ctx.FlagValue(flag)
	if val != nil {
		// Format based on type
		switch v := val.(type) {
		case bool:
			fmt.Fprintf(w, "  %s : %t,\n", name, v)
		case string:
			if v != "" {
				fmt.Fprintf(w, "  %s : %q,\n", name, v)
			}
		default:
			fmt.Fprintf(w, "  %s : %q,\n", name, fmt.Sprint(v))
		}
	}
}
