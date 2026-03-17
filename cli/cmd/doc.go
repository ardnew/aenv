// Package cmd provides the CLI subcommands for the aenv application.
package cmd

var (
	// CacheIdentifier is the kong variable identifier containing the path to
	// the runtime cache directory.
	CacheIdentifier = "cache"

	// ConfigIdentifier is the kong variable identifier containing the name of
	// the default configuration namespace parsed from the configuration file.
	ConfigIdentifier = "config"
)
