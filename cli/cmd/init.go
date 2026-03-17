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
	Force bool `help:"Overwrite existing configuration file" negatable:"" short:"w"`
}

// Run executes the init command.
func (i *Init) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)

	defer func(err *error) { cancel(*err) }(&err)

	ktx := kongContextFrom(ctx)

	confPath, ok := ktx.Model.Vars()[ConfigIdentifier]
	if !ok {
		return ErrMissingConfig
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

	fmt.Printf("Configuration written to %s\n", confPath)

	return nil
}

// buildAST constructs the config AST from current flag values.
func (i *Init) buildAST(ctx context.Context) *lang.AST {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ktx := kongContextFrom(ctx)

	var entries []*lang.Namespace

	prefixIgnore := []string{"help", "verbose", "quiet", profile.Tag}

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
	ast.DefineNamespace(ConfigIdentifier, nil, lang.NewBlock(entries...))

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
		return lang.NewExpr(strconv.FormatBool(v))

	case string:
		if v == "" {
			return nil
		}

		return lang.NewExpr(strconv.Quote(v))

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return lang.NewExpr(fmt.Sprint(v))

	case float32, float64:
		return lang.NewExpr(fmt.Sprint(v))

	case []string:
		return sliceExpr(v, strconv.Quote)

	case []int:
		return sliceExpr(v, strconv.Itoa)

	case []int64:
		return sliceExpr(v, func(n int64) string {
			return strconv.FormatInt(n, 10)
		})

	case []uint:
		return sliceExpr(v, func(n uint) string {
			return strconv.FormatUint(uint64(n), 10)
		})

	case []uint64:
		return sliceExpr(v, func(n uint64) string {
			return strconv.FormatUint(n, 10)
		})

	case []float32:
		return sliceExpr(v, func(f float32) string {
			return strconv.FormatFloat(float64(f), 'f', -1, 32)
		})

	case []float64:
		return sliceExpr(v, func(f float64) string {
			return strconv.FormatFloat(f, 'f', -1, 64)
		})

	case []bool:
		return sliceExpr(v, strconv.FormatBool)

	default:
		return lang.NewExpr(strconv.Quote(fmt.Sprint(v)))
	}
}

func sliceExpr[T any](v []T, toString func(T) string) *lang.Value {
	if len(v) == 0 {
		return nil
	}

	elems := make([]string, len(v))
	for i, val := range v {
		elems[i] = toString(val)
	}

	return lang.NewExpr("[" + strings.Join(elems, ", ") + "]")
}
