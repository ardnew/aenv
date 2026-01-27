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
func (ast *AST) Format(w io.Writer, indent int) error {
	count := 0
	for _, def := range ast.Definitions {
		if count > 0 {
			// Delimit top-level definitions with semicolon
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

		err := formatDefinition(def, w, indent, 0)
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
func (ast *AST) FormatJSON(w io.Writer, indent int) error {
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

// formatDefinition formats a definition in native aenv syntax.
func formatDefinition(def *Definition, w io.Writer, indent, depth int) error {
	if _, err := fmt.Fprint(w, def.Identifier.LiteralString()); err != nil {
		return err
	}

	if len(def.Parameters) > 0 {
		for _, param := range def.Parameters {
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

	return formatValue(def.Value, w, indent, depth)
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

	case TypeDefinition:
		if v.Definition != nil {
			return formatDefinition(v.Definition, w, indent, depth)
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

	if len(t.Aggregate) > 0 && indent > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	for i, val := range t.Aggregate {
		// Indent
		if _, err := fmt.Fprint(w, strings.Repeat(" ", (depth+1)*indent)); err != nil {
			return err
		}

		// If this value is a Definition, write it as key : value
		if val.Type == TypeDefinition && val.Definition != nil {
			if _, err := fmt.Fprint(w, val.Definition.Identifier.LiteralString(), " : "); err != nil {
				return err
			}

			err := formatValue(val.Definition.Value, w, indent, depth+1)
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
			if i < len(t.Aggregate)-1 {
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
