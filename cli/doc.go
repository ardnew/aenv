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
// The package uses the [lang] package to parse aenv configuration files into
// an [lang.AST]:
//
//   - [lang.ParseReader]: Parse an [lang.AST] from an [io.Reader]
//   - [lang.ParseString]: Parse an [lang.AST] from a string
//   - [lang.AST.GetNamespace]: Retrieve a namespace by name
//   - [lang.AST.All]: Iterate over all namespaces
//
// - [lang.AST.EvaluateNamespace]: Evaluate a namespace with optional arguments
//
// Example usage:
//
//	ast, err := lang.ParseString(ctx, `config : { level : "debug" }`)
//	ns, ok := ast.GetNamespace("config")
//
// # Configuration Loader
//
// The package includes a Kong configuration loader that reads aenv language
// config files and converts them to Kong flag values.
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
//		go build -tags pprof -o aenv
//
//	  - --pprof-mode: Enable profiling (allocs, block, clock, cpu, goroutine,
//	    heap, mem, mutex, thread, trace)
//	  - --pprof-dir: Set profile output directory
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
