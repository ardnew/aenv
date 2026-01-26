// Package cli contains the command line interface for aenv.
//
// # Usage
//
// The CLI provides logging and profiling configuration:
//
//	aenv --log-level=debug --pprof-mode=cpu
//
// # Parser
//
// The package uses the lang package's streaming parser with both method-based
// and functional interfaces for efficient access to definitions:
//
// Method-based interface (recommended):
//   - [lang.NewStream]: Create a parser from an io.Reader
//   - [lang.NewStreamFromString]: Create a parser from a string
//   - [lang.Stream.GetDefinition]: Retrieve a specific definition by identifier
//   - [lang.Stream.Definitions]: Iterate over all definitions using iter.Seq
//   - [lang.Stream.AST]: Access the complete parsed AST
//
// Functional interface:
// - [lang.GetDefinitionFrom]: Directly retrieve a definition from an io.Reader
// - [lang.DefinitionsFrom]: Get an iterator over definitions from an io.Reader
//
// Utility:
//   - [lang.ClearCache]: Clear all cached ASTs (useful for testing)
//
// The parser caches parsed ASTs by source content, ensuring identical content
// is parsed only once even when accessed from multiple goroutines.
//
// Example usage:
//
//	// Method-based streaming interface
//	p := lang.NewStreamFromString(`config : { level = "debug" }`)
//	def, err := p.GetDefinition("config")
//
//	// Iterate over all definitions
//	for def := range p.Definitions() {
//	    fmt.Println(def.Identifier.LiteralString())
//	}
//
//	// Functional interface
//	def, err := lang.GetDefinitionFrom(reader, "config")
//
// # Configuration Loader
//
// The package includes a Kong configuration loader ([loadNamespace]) that
// reads aenv language config files and converts them to Kong flag values.
//
// # Logging Options
//
//   - --log-level: Set minimum log level (debug, info, warn, error)
//   - --log-format: Set log output format (json, text)
//   - --log-time: Set timestamp format (RFC3339, RFC3339Nano, etc.)
//   - --log-caller: Include caller information in log output
//
// # Profiling Options
//
// Profiling is only available when built with the pprof build tag:
//
//		go build -tags pprof -o aenv ./cmd/aenv
//
//	  - --pprof-mode: Enable profiling (allocs, block, clock, cpu, goroutine,
//	    heap, mem, mutex, thread, trace)
//	  - --pprof-dir: Set profile output directory (default:
//
// ~/.cache/aenv/pprof)
//
// # Examples
//
//	# Debug logging with CPU profiling
//	aenv --log-level=debug --pprof-mode=cpu
//
//	# Text format with heap profiling
//	aenv --log-format=text --pprof-mode=heap
//
//	# Custom profile directory
//	aenv --pprof-mode=allocs --pprof-dir=/tmp/profiles
package cli
