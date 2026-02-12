package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/profile"
)

// defaultConfigIndent is the number of spaces to use for indentation
// when generating the default configuration file.
const defaultConfigIndent = 2

// Init generates a default configuration file with current flag values.
type Init struct {
	Force bool `help:"Overwrite existing configuration file" short:"f"`
}

// Run executes the init command.
func (i *Init) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)

	defer func(err *error) { cancel(*err) }(&err)

	ktx := kongContextFrom(ctx)

	confPath, ok := ktx.Model.Vars()[ConfigIdentifier]
	if !ok {
		panic("internal error: config namespace undefined")
	}

	// Check if file exists and force not set
	_, err = os.Stat(confPath)
	if err == nil && !i.Force {
		return ErrWriteConfig.
			With(slog.String("file", confPath)).
			With(slog.Bool("exists", true)).
			Wrap(ErrFileExists)
	}

	file, err := os.Create(confPath)
	if err != nil {
		return ErrWriteConfig.
			With(slog.String("file", confPath)).
			Wrap(err)
	}
	defer file.Close()

	ast := i.buildAST(ctx)

	err = ast.Format(ctx, file, defaultConfigIndent)
	if err != nil {
		return ErrWriteConfig.
			With(slog.String("file", confPath)).
			Wrap(err)
	}

	log.DebugContext(
		ctx,
		"initialized configuration file",
		slog.String("path", confPath),
	)

	return nil
}

// buildAST constructs the config AST from current flag values.
func (i *Init) buildAST(ctx context.Context) *lang.AST {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ktx := kongContextFrom(ctx)

	var entries []*lang.Value

	prefixIgnore := []string{"help", profile.Tag}

	for _, flag := range ktx.Model.Flags {
		if flag.Hidden || slices.ContainsFunc(prefixIgnore, func(s string) bool {
			return strings.HasPrefix(flag.Name, s)
		}) {
			continue
		}

		val := i.flagValue(ctx, flag.Name)
		if val != nil {
			entries = append(entries, lang.NewNamespace(flag.Name, nil, val))
		}
	}

	ast := new(lang.AST)
	ast.DefineNamespace(ConfigIdentifier, nil, lang.NewTuple(entries...))

	return ast
}

// flagValue returns the AST value for a CLI flag, or nil if unset.
func (i *Init) flagValue(ctx context.Context, name string) *lang.Value {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ktx := kongContextFrom(ctx)

	idx := slices.IndexFunc(ktx.Model.Flags, func(flag *kong.Flag) bool {
		return flag.Name == name
	})
	if idx == -1 {
		return nil
	}

	val := ktx.FlagValue(ktx.Model.Flags[idx])
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

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return lang.NewNumber(fmt.Sprint(v))

	case float32, float64:
		return lang.NewNumber(fmt.Sprint(v))

	case []string:
		if len(v) == 0 {
			return nil
		}

		entries := make([]*lang.Value, len(v))
		for i, s := range v {
			entries[i] = lang.NewString(s)
		}

		return lang.NewTuple(entries...)

	case []int:
		if len(v) == 0 {
			return nil
		}

		entries := make([]*lang.Value, len(v))
		for i, n := range v {
			entries[i] = lang.NewNumber(strconv.Itoa(n))
		}

		return lang.NewTuple(entries...)

	case []int64:
		if len(v) == 0 {
			return nil
		}

		entries := make([]*lang.Value, len(v))
		for i, n := range v {
			entries[i] = lang.NewNumber(strconv.FormatInt(n, 10))
		}

		return lang.NewTuple(entries...)

	case []float64:
		if len(v) == 0 {
			return nil
		}

		entries := make([]*lang.Value, len(v))
		for i, n := range v {
			entries[i] = lang.NewNumber(fmt.Sprint(n))
		}

		return lang.NewTuple(entries...)

	case []bool:
		if len(v) == 0 {
			return nil
		}

		entries := make([]*lang.Value, len(v))
		for i, b := range v {
			entries[i] = lang.NewBool(b)
		}

		return lang.NewTuple(entries...)

	default:
		return lang.NewString(fmt.Sprint(v))
	}
}
