// Package cli contains the command line interface for envcomp.
//
// # Usage
//
// The CLI provides logging and profiling configuration:
//
//	envcomp --log-level=debug --pprof-mode=cpu
//
// # Parser
//
// The package provides a streaming parser with both method-based and functional
// interfaces for efficient access to namespaces:
//
// Method-based interface (recommended):
//   - [NewParser]: Create a parser from an io.Reader
//   - [NewParserFromString]: Create a parser from a string
//   - [Parser.GetNamespace]: Retrieve a specific namespace by identifier
//   - [Parser.Namespaces]: Iterate over all namespaces using iter.Seq
//   - [Parser.AST]: Access the complete parsed AST
//
// Functional interface:
//   - [GetNamespaceFrom]: Directly retrieve a namespace from an io.Reader
//   - [NamespacesFrom]: Get an iterator over namespaces from an io.Reader
//
// Utility:
//   - [ClearCache]: Clear all cached ASTs (useful for testing)
//
// The parser caches parsed ASTs by source content, ensuring identical content
// is parsed only once even when accessed from multiple goroutines.
//
// Example usage:
//
//	// Method-based streaming interface
//	p := cli.NewParserFromString(`config : { level = "debug" }`)
//	ns, err := p.GetNamespace("config")
//
//	// Iterate over all namespaces
//	for ns := range p.Namespaces() {
//	    fmt.Println(ns.Identifier.LiteralString())
//	}
//
//	// Functional interface
//	ns, err := cli.GetNamespaceFrom(reader, "config")
//
// # Configuration Loader
//
// The package includes a Kong configuration loader ([loadNamespace]) that
// reads envcomp language config files and converts them to Kong flag values.
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
//		go build -tags pprof -o envcomp ./cmd/envcomp
//
//	  - --pprof-mode: Enable profiling (allocs, block, clock, cpu, goroutine,
//	    heap, mem, mutex, thread, trace)
//	  - --pprof-dir: Set profile output directory (default:
//
// ~/.cache/envcomp/pprof)
//
// # Examples
//
//	# Debug logging with CPU profiling
//	envcomp --log-level=debug --pprof-mode=cpu
//
//	# Text format with heap profiling
//	envcomp --log-format=text --pprof-mode=heap
//
//	# Custom profile directory
//	envcomp --pprof-mode=allocs --pprof-dir=/tmp/profiles
package cli
