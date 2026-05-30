// Package cli defines the command-line interface syntax.
//
// The --log flag adds log handlers using the form OUTPUT[,FORMAT[,LEVEL]]. It
// may be repeated. OUTPUT is "-" for stdout, "stderr" for stderr, or a file
// path; an empty OUTPUT defaults to stdout. FORMAT is "text" or "json" and
// defaults to text for terminals and JSON for files. LEVEL is "error", "warn",
// "info", "debug", or "trace" and defaults to "info". A console --log handler
// replaces the built-in stdout text warn handler; file handlers supplement it.
//
// The version subcommand prints version metadata directly to stdout. Passing
// --semantic or -s prints only the strict semantic version string. Passing
// --url prints the project URL, and passing --repo prints the repository URL.
package cli
