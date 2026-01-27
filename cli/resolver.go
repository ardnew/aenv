package cli

import (
	"io"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/lang"
)

// Load is a [kong.ConfigurationLoader] that parses config files written in
// the aenv language format.
//
// It can be used with [kong.Configuration] like this:
//
//	kong.Configuration(load, "/path/to/config")
//
// The aenv language structure is converted as follows:
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
// Example aenv config file:
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
func resolve(name string) func(r io.Reader) (kong.Resolver, error) {
	return func(r io.Reader) (kong.Resolver, error) {
		// Parse the config file (cached after first parse)
		ast, err := lang.ParseReader(r)
		if err != nil {
			// Parse error - return empty config
			return config{}, nil
		}

		def, ok := ast.GetDefinition(name)
		if !ok {
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

// config implements [kong.Resolver] for aenv language configs.
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
	// Kong flags use hyphens (e.g., "log-level") but aenv identifiers
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
			nativeValue := val.Definition.Value.ToNative()

			// Kong requires numbers as strings for parsing
			if num, ok := nativeValue.(int64); ok {
				result[key] = strconv.FormatInt(num, 10)
			} else if num, ok := nativeValue.(float64); ok {
				result[key] = strconv.FormatFloat(num, 'f', -1, 64)
			} else {
				result[key] = nativeValue
			}
		} else {
			// Otherwise, it's a value without a key (shouldn't happen in map context)
			// but handle it gracefully by using the index
			result[strconv.Itoa(len(result))] = val.ToNative()
		}
	}

	return result
}
