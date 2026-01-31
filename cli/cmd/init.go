package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

const (
	// ConfigNamespace is the kong variable identifier containing the name of
	// the default configuration namespace parsed from the configuration file.
	ConfigNamespace = "config"

	// defaultConfigIndent is the number of spaces to use for indentation
	// when generating the default configuration file.
	defaultConfigIndent = 2
)

// Init generates a default configuration file with current flag values.
type Init struct {
	Force bool `help:"Overwrite existing configuration file" short:"f"`
}

// Run executes the init command.
func (i *Init) Run(ctx *kong.Context) error {
	conf, ok := ctx.Model.Vars()[ConfigNamespace]
	if !ok {
		panic("internal error: config namespace undefined")
	}

	// Check if file exists and force not set
	_, err := os.Stat(conf)
	if err == nil && !i.Force {
		return ErrWriteConfig.
			With(slog.String("file", conf)).
			With(slog.Bool("exists", true)).
			Wrap(ErrFileExists)
	}

	file, err := os.Create(conf)
	if err != nil {
		return ErrWriteConfig.
			With(slog.String("file", conf)).
			Wrap(err)
	}
	defer file.Close()

	ast := i.buildAST(ctx)

	err = ast.Format(file, defaultConfigIndent)
	if err != nil {
		return ErrWriteConfig.
			With(slog.String("file", conf)).
			Wrap(err)
	}

	log.Debug("initialized configuration file", slog.String("path", conf))

	return nil
}

// buildAST constructs the config AST from current flag values.
func (i *Init) buildAST(ctx *kong.Context) *lang.AST {
	flags := []string{
		"log-level",
		"log-format",
		"log-time-layout",
		"log-caller",
		"log-pretty",
		"pprof-mode",
		"pprof-dir",
	}

	var entries []*lang.Value

	for _, name := range flags {
		val := i.flagValue(ctx, name)
		if val != nil {
			entries = append(entries, lang.NewDefinition(name, nil, val))
		}
	}

	ast := new(lang.AST)
	ast.Define(ConfigNamespace, nil, lang.NewTuple(entries...))

	return ast
}

// flagValue returns the AST value for a CLI flag, or nil if unset.
func (i *Init) flagValue(ctx *kong.Context, name string) *lang.Value {
	idx := slices.IndexFunc(ctx.Model.Flags, func(flag *kong.Flag) bool {
		return flag.Name == name
	})
	if idx == -1 {
		return nil
	}

	val := ctx.FlagValue(ctx.Model.Flags[idx])
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case bool:
		return lang.NewBool(v)
	case string:
		if v == "" {
			return nil
		}

		return lang.NewString(v)
	default:
		return lang.NewString(fmt.Sprint(v))
	}
}
