package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/ardnew/aenv/lang"
)

// Fmt reads input from stdin, parses it, and formats it in the chosen format.
type Fmt struct {
	Native Native `cmd:"" default:"withargs" help:"Format as native aenv syntax (default)."`
	JSON   JSON   `cmd:""                    help:"Format as JSON."`
	YAML   YAML   `cmd:""                    help:"Format as YAML."`
	AST    AST    `cmd:""                    help:"Format as abstract syntax tree."`
}

// Native formats input as native aenv syntax.
type Native struct {
	Indent int `default:"2" help:"Indent width for formatted output" short:"i"`

	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the fmt command.
func (f *Native) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if f.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(f.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	p := lang.NewStream(bufio.NewReader(file))

	ast, err := p.AST()
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "native"))
	}

	f.formatAST(ast, os.Stdout)

	return nil
}

// formatAST formats the AST in native aenv language syntax.
func (f *Native) formatAST(ast *lang.AST, w io.Writer) {
	count := 0
	for _, def := range ast.Definitions {
		if count > 0 {
			// Delimit top-level definitions with semicolon
			fmt.Fprint(w, ";")

			if f.Indent > 0 {
				fmt.Fprintln(w) // Blank line between definitions
				fmt.Fprintln(w)
			} else {
				fmt.Fprint(w, " ") // Space between definitions
			}
		}

		f.formatDefinition(def, w, 0)

		count++
	}

	fmt.Fprintln(w) // Final newline for non-indented output
}

// formatTuple formats a tuple.
func (f *Native) formatTuple(t *lang.Tuple, w io.Writer, depth int) {
	fmt.Fprint(w, "{")

	if len(t.Aggregate) > 0 && f.Indent > 0 {
		fmt.Fprintln(w)
	}

	for i, val := range t.Aggregate {
		// Indent
		fmt.Fprint(w, strings.Repeat(" ", (depth+1)*f.Indent))

		// If this value is a Definition, write it as key : value
		if val.Type == lang.TypeDefinition && val.Definition != nil {
			fmt.Fprint(w, val.Definition.Identifier.LiteralString(), " : ")
			f.formatValue(val.Definition.Value, w, depth+1)
		} else {
			// Otherwise just write the value
			f.formatValue(val, w, depth+1)
		}

		if f.Indent == 0 {
			if i < len(t.Aggregate)-1 {
				fmt.Fprint(w, ", ")
			}
		} else {
			// Always add comma for easier editing
			fmt.Fprintln(w, ",")
		}
	}

	// Closing brace
	fmt.Fprint(w, strings.Repeat(" ", depth*f.Indent), "}")
}

func (f *Native) formatDefinition(
	def *lang.Definition,
	w io.Writer,
	depth int,
) {
	fmt.Fprint(w, def.Identifier.LiteralString())

	if len(def.Parameters) > 0 {
		for _, param := range def.Parameters {
			fmt.Fprint(w, " ")
			f.formatValue(param, w, depth)
		}
	}

	fmt.Fprint(w, " : ")
	f.formatValue(def.Value, w, depth)
}

// formatValue formats a value based on its type.
func (f *Native) formatValue(v *lang.Value, w io.Writer, depth int) {
	switch v.Type {
	case lang.TypeIdentifier,
		lang.TypeNumber,
		lang.TypeString,
		lang.TypeExpr,
		lang.TypeBoolean:
		fmt.Fprint(w, v.Token.LiteralString())

	case lang.TypeTuple:
		if v.Tuple != nil {
			f.formatTuple(v.Tuple, w, depth)
		}

	case lang.TypeDefinition:
		// Recursive definition: format inline
		if v.Definition != nil {
			f.formatDefinition(v.Definition, w, depth)
		}

	default:
		fmt.Fprint(w, "<unknown>")
	}
}

// JSON reads input from stdin, parses it, and outputs as JSON.
type JSON struct {
	Indent int `default:"2" help:"Indent width for JSON output" short:"i"`

	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the json command.
func (j *JSON) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if j.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(j.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	p := lang.NewStream(bufio.NewReader(file))

	ast, err := p.AST()
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "json"))
	}

	// Marshal to JSON
	var jsonData []byte
	if j.Indent > 0 {
		jsonData, err = json.MarshalIndent(ast, "", strings.Repeat(" ", j.Indent))
	} else {
		jsonData, err = json.Marshal(ast)
	}

	if err != nil {
		return ErrJSONMarshal.
			With(slog.Int("indent", j.Indent)).
			Wrap(err)
	}

	// Write to stdout
	fmt.Println(string(jsonData))

	return nil
}

// YAML reads input from stdin, parses it, and outputs as YAML.
type YAML struct {
	Indent int `default:"2" help:"Indent width for YAML output" short:"i"`

	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the yaml command.
func (y *YAML) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if y.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(y.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	p := lang.NewStream(bufio.NewReader(file))

	ast, err := p.AST()
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "yaml"))
	}

	var opts []yaml.EncodeOption
	if y.Indent > 0 {
		opts = append(opts, yaml.Indent(y.Indent))
	} else {
		opts = append(opts, yaml.Flow(true))
	}

	// Marshal to YAML
	yamlData, err := yaml.MarshalContext(ctx, ast.ToMap(), opts...)
	if err != nil {
		return ErrYAMLMarshal.
			With(slog.Int("indent", y.Indent)).
			Wrap(err)
	}

	// Write to stdout
	fmt.Print(string(yamlData))

	return nil
}

// AST formats input as an abstract syntax tree representation.
type AST struct {
	Source string `arg:"" default:"-" help:"Source input file or '-' for default stdin." name:"source"`
}

// Run executes the ast command.
func (a *AST) Run(ctx context.Context) (err error) {
	_, cancel := context.WithCancelCause(ctx)

	defer func(err *error) {
		cancel(*err)
	}(&err)

	var file *os.File
	if a.Source == "-" {
		file = os.Stdin
	} else {
		var err error

		file, err = os.Open(a.Source)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	p := lang.NewStream(bufio.NewReader(file))

	ast, err := p.AST()
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("format", "ast"))
	}

	// Print the AST to stdout
	ast.Print(os.Stdout)

	return nil
}
