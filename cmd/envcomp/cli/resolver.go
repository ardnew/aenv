package cli

import (
	"io"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/envcomp/cmd/envcomp/parser"
	"github.com/ardnew/envcomp/pkg/lang"
)

// Load is a [kong.ConfigurationLoader] that parses config files written in
// the envcomp language format.
//
// It can be used with [kong.Configuration] like this:
//
//	kong.Configuration(load, "/path/to/config")
//
// The envcomp language structure is converted as follows:
//   - Each definition scope becomes a flat configuration map
//   - Definition parameters are ignored (used for environment composition)
//   - Flag names with hyphens (e.g., "log-level") should use underscores
//     in the config file (e.g., "log_level")
//   - Tuples are converted to nested objects
//   - Aggregates are converted to arrays
//   - String values should be quoted
//   - Boolean values are true or false (unquoted)
//   - Numbers are unquoted
//
// Example envcomp config file:
//
//	config : {
//	  log_level = "debug",
//	  log_format = "json",
//	  log_pretty = true
//	}
//
// This configuration will be applied to Kong flags:
//
//	--log-level=debug
//	--log-format=json
//	--log-pretty=true
//
// Command-line flags override config file values.
func loadNamespace(name string) func(r io.Reader) (kong.Resolver, error) {
	return func(r io.Reader) (kong.Resolver, error) {
		// Parse the config file (cached after first parse)
		p := parser.New(r)

		def, err := p.GetDefinition(name)
		if err != nil {
			// Definition not found - return empty config
			return config{}, nil
		}

		if def.Value.Type != lang.TypeTuple {
			// Not a tuple - return empty config
			return config{}, nil
		}

		return config(tupleToMap(def.Value.Tuple)), nil
	}
}

// config implements [kong.Resolver] for envcomp language configs.
type config map[string]any

// Validate implements [kong.Resolver].
func (r config) Validate(*kong.Application) error {
	// No validation needed - the config was already parsed successfully
	return nil
}

// Resolve implements [kong.Resolver].
func (r config) Resolve(
	_ *kong.Context,
	_ *kong.Path,
	flag *kong.Flag,
) (any, error) {
	// Kong flags use hyphens (e.g., "log-level") but envcomp identifiers
	// may use underscores. Try both forms.
	name := flag.Name
	underscoreName := strings.ReplaceAll(name, "-", "_")

	// Look up the value in our config
	if value, ok := r[name]; ok {
		return value, nil
	}

	// Try underscore variant
	if value, ok := r[underscoreName]; ok {
		return value, nil
	}

	// Not found - return nil to let Kong use defaults
	return nil, nil
}

// tupleToMap converts a Tuple to a native map representation.
func tupleToMap(t *lang.Tuple) map[string]any {
	result := make(map[string]any)

	for _, val := range t.Aggregate {
		// If this value is a Definition, use its identifier as the key
		if val.Type == lang.TypeDefinition && val.Definition != nil {
			key := val.Definition.Identifier.LiteralString()
			result[key] = valueToNative(val.Definition.Value)
		} else {
			// Otherwise, it's a value without a key (shouldn't happen in map context)
			// but handle it gracefully by using the index
			result[strconv.Itoa(len(result))] = valueToNative(val)
		}
	}

	return result
}

// isAggregate checks if a Tuple represents an aggregate (array) rather than
// a structured tuple (map). Aggregates have values that are not Definitions.
func isAggregate(t *lang.Tuple) bool {
	if t == nil || len(t.Aggregate) == 0 {
		return false
	}
	// Check if all values are NOT Definitions (no keys)
	for _, val := range t.Aggregate {
		if val.Type == lang.TypeDefinition {
			return false // Has a key-value pair, so it's a map
		}
	}

	return true // All plain values, so it's an array
}

// valueToNative converts a Value to its native Go type for configuration.
func valueToNative(v *lang.Value) any {
	switch v.Type {
	case lang.TypeIdentifier:
		return v.Token.LiteralString()

	case lang.TypeNumber:
		// Kong will parse numbers from strings as needed
		return v.Token.LiteralString()

	case lang.TypeString:
		// Remove quotes from string literals
		s := v.Token.LiteralString()
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			// Unquote the string (escape sequences handled by lang.ParseString)
			s = s[1 : len(s)-1]
		}

		return s

	case lang.TypeExpr:
		// Return code as-is (enclosed in {{ }})
		return v.Token.LiteralString()

	case lang.TypeBoolean:
		s := v.Token.LiteralString()

		return s == "true"

	case lang.TypeTuple:
		// Check if this is an aggregate (array) or a structured tuple (map)
		// Aggregates have values that are not Definitions
		if isAggregate(v.Tuple) {
			result := make([]any, len(v.Tuple.Aggregate))
			for i, val := range v.Tuple.Aggregate {
				result[i] = valueToNative(val)
			}

			return result
		}

		return tupleToMap(v.Tuple)

	case lang.TypeDefinition:
		// Recursively handle definitions - treat as tuples if the value is one
		if v.Definition != nil && v.Definition.Value != nil {
			return valueToNative(v.Definition.Value)
		}

		return nil

	default:
		return nil
	}
}
