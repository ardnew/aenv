package lang

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
)

// Format writes the AST in native lang2 syntax to the writer.
func (a *AST) Format(_ context.Context, w io.Writer, indent int) error {
	count := 0
	for _, ns := range a.Namespaces {
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

	// Always add trailing semicolon after the last namespace for consistency
	if len(a.Namespaces) > 0 {
		if _, err := fmt.Fprint(w, ";"); err != nil {
			return err
		}
	}

	// Final newline
	_, err := fmt.Fprintln(w)

	return err
}

// FormatJSON writes the AST as JSON to the writer.
func (a *AST) FormatJSON(_ context.Context, w io.Writer, indent int) error {
	var (
		jsonData []byte
		err      error
	)

	if indent > 0 {
		jsonData, err = json.MarshalIndent(a, "", strings.Repeat(" ", indent))
	} else {
		jsonData, err = json.Marshal(a)
	}

	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(w, string(jsonData))

	return err
}

// FormatYAML writes the AST as YAML to the writer.
func (a *AST) FormatYAML(ctx context.Context, w io.Writer, indent int) error {
	var opts []yaml.EncodeOption
	if indent > 0 {
		opts = append(opts, yaml.Indent(indent))
	} else {
		opts = append(opts, yaml.Flow(true))
	}

	// Marshal to YAML
	yamlData, err := yaml.MarshalContext(
		ctx,
		a.ToMap(),
		opts...)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, string(yamlData))

	return err
}

// Print writes a debug representation of the AST to the writer.
// This is useful for inspecting the AST structure.
func (a *AST) Print(_ context.Context, w io.Writer) error {
	fmt.Fprintf(w, "AST with %d namespaces:\n", len(a.Namespaces))

	for i, ns := range a.Namespaces {
		fmt.Fprintf(w, "  [%d] %s", i, ns.Name)

		if len(ns.Params) > 0 {
			fmt.Fprintf(w, " (")

			for j, p := range ns.Params {
				if j > 0 {
					fmt.Fprintf(w, ", ")
				}

				if p.Variadic {
					fmt.Fprintf(w, "...")
				}

				fmt.Fprintf(w, "%s", p.Name)
			}

			fmt.Fprintf(w, ")")
		}

		fmt.Fprintf(w, " : ")

		if ns.Value != nil {
			fmt.Fprintf(w, "%s\n", ns.Value.Kind)
		} else {
			fmt.Fprintf(w, "nil\n")
		}
	}

	return nil
}

// formatNamespace formats a namespace in native lang2 syntax.
func formatNamespace(ns *Namespace, w io.Writer, indent, depth int) error {
	if _, err := fmt.Fprint(w, ns.Name); err != nil {
		return err
	}

	// Format parameters
	if len(ns.Params) > 0 {
		last := len(ns.Params) - 1
		for i, param := range ns.Params {
			if _, err := fmt.Fprint(w, " "); err != nil {
				return err
			}

			// Prefix the variadic (last) parameter with "..."
			if i == last && param.Variadic {
				if _, err := fmt.Fprint(w, "..."); err != nil {
					return err
				}
			}

			if _, err := fmt.Fprint(w, param.Name); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprint(w, " : "); err != nil {
		return err
	}

	return formatValue(ns.Value, w, indent, depth)
}

// formatValue formats a value based on its kind.
func formatValue(v *Value, w io.Writer, indent, depth int) error {
	if v == nil {
		_, err := fmt.Fprint(w, "nil")

		return err
	}

	switch v.Kind {
	case KindExpr:
		// Expression: write the raw source
		_, err := fmt.Fprint(w, v.Source)

		return err

	case KindBlock:
		// Block: format as { namespace, ... }
		return formatBlock(v, w, indent, depth)

	default:
		_, err := fmt.Fprint(w, "<unknown>")

		return err
	}
}

// formatBlock formats a block value.
func formatBlock(v *Value, w io.Writer, indent, depth int) error {
	if _, err := fmt.Fprint(w, "{"); err != nil {
		return err
	}

	if len(v.Entries) > 0 && indent > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	for i, ns := range v.Entries {
		// Indent
		if indent > 0 {
			if _, err := fmt.Fprint(w, strings.Repeat(" ", (depth+1)*indent)); err != nil {
				return err
			}
		}

		// Format namespace entry
		if err := formatNamespace(ns, w, indent, depth+1); err != nil {
			return err
		}

		// Add separator (semicolon)
		if i < len(v.Entries)-1 {
			// Not the last entry: always add semicolon
			if indent == 0 {
				// Compact mode: semicolon + space
				if _, err := fmt.Fprint(w, "; "); err != nil {
					return err
				}
			} else {
				// Pretty mode: semicolon + newline
				if _, err := fmt.Fprintln(w, ";"); err != nil {
					return err
				}
			}
		} else {
			// Last entry: always emit trailing semicolon for consistency
			if _, err := fmt.Fprint(w, ";"); err != nil {
				return err
			}

			if indent > 0 {
				// Pretty mode: add newline after semicolon
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
		}
	}

	// Closing brace
	if indent > 0 && len(v.Entries) > 0 {
		if _, err := fmt.Fprint(w, strings.Repeat(" ", depth*indent)); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, "}")

	return err
}
