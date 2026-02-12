package lang

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
)

// Format writes the AST in native aenv language syntax to the writer.
func (ast *AST) Format(_ context.Context, w io.Writer, indent int) error {
	count := 0
	for _, ns := range ast.Namespaces {
		if count > 0 {
			// Delimit top-level namespaces with semicolon
			if _, err := fmt.Fprint(w, ";"); err != nil {
				return err
			}

			if indent > 0 {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}

				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprint(w, " "); err != nil {
					return err
				}
			}
		}

		err := formatNamespace(ns, w, indent, 0)
		if err != nil {
			return err
		}

		count++
	}

	// Final newline
	_, err := fmt.Fprintln(w)

	return err
}

// FormatJSON writes the AST as JSON to the writer.
func (ast *AST) FormatJSON(_ context.Context, w io.Writer, indent int) error {
	var (
		jsonData []byte
		err      error
	)

	if indent > 0 {
		jsonData, err = json.MarshalIndent(ast, "", strings.Repeat(" ", indent))
	} else {
		jsonData, err = json.Marshal(ast)
	}

	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(w, string(jsonData))

	return err
}

// FormatYAML writes the AST as YAML to the writer.
func (ast *AST) FormatYAML(ctx context.Context, w io.Writer, indent int) error {
	var opts []yaml.EncodeOption
	if indent > 0 {
		opts = append(opts, yaml.Indent(indent))
	} else {
		opts = append(opts, yaml.Flow(true))
	}

	// Marshal to YAML
	yamlData, err := yaml.MarshalContext(
		ctx,
		ast.ToMap(),
		opts...)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, string(yamlData))

	return err
}

// formatNamespace formats a namespace in native aenv syntax.
func formatNamespace(ns *Namespace, w io.Writer, indent, depth int) error {
	if _, err := fmt.Fprint(w, ns.Identifier.LiteralString()); err != nil {
		return err
	}

	if len(ns.Parameters) > 0 {
		for _, param := range ns.Parameters {
			if _, err := fmt.Fprint(w, " "); err != nil {
				return err
			}

			err := formatValue(param, w, indent, depth)
			if err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprint(w, " : "); err != nil {
		return err
	}

	return formatValue(ns.Value, w, indent, depth)
}

// formatValue formats a value based on its type.
func formatValue(v *Value, w io.Writer, indent, depth int) error {
	switch v.Type {
	case TypeIdentifier, TypeNumber, TypeString, TypeExpr, TypeBoolean:
		_, err := fmt.Fprint(w, v.Token.LiteralString())

		return err

	case TypeTuple:
		if v.Tuple != nil {
			return formatTuple(v.Tuple, w, indent, depth)
		}

		return nil

	case TypeNamespace:
		if v.Namespace != nil {
			return formatNamespace(v.Namespace, w, indent, depth)
		}

		return nil

	default:
		_, err := fmt.Fprint(w, "<unknown>")

		return err
	}
}

// formatTuple formats a tuple.
func formatTuple(t *Tuple, w io.Writer, indent, depth int) error {
	if _, err := fmt.Fprint(w, "{"); err != nil {
		return err
	}

	if len(t.Values) > 0 && indent > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	for i, val := range t.Values {
		// Indent
		if _, err := fmt.Fprint(w, strings.Repeat(" ", (depth+1)*indent)); err != nil {
			return err
		}

		// If this value is a Namespace, write it as key : value
		if val.Type == TypeNamespace && val.Namespace != nil {
			if _, err := fmt.Fprint(w, val.Namespace.Identifier.LiteralString(), " : "); err != nil {
				return err
			}

			err := formatValue(val.Namespace.Value, w, indent, depth+1)
			if err != nil {
				return err
			}
		} else {
			// Otherwise just write the value
			err := formatValue(val, w, indent, depth+1)
			if err != nil {
				return err
			}
		}

		if indent == 0 {
			if i < len(t.Values)-1 {
				if _, err := fmt.Fprint(w, ", "); err != nil {
					return err
				}
			}
		} else {
			// Always add comma for easier editing
			if _, err := fmt.Fprintln(w, ","); err != nil {
				return err
			}
		}
	}

	// Closing brace
	_, err := fmt.Fprint(w, strings.Repeat(" ", depth*indent), "}")

	return err
}
