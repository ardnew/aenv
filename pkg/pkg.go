//nolint:gochecknoglobals
package pkg

import (
	_ "embed"
)

// Version is the semantic version of the aenv module embedded at build time.
// It is printed by the CLI when users invoke the version subcommand.
//
//go:embed VERSION
var Version string

const (
	// Name is the canonical command and module identifier used across the
	// project. For example, it appears in help text and default config paths.
	Name = "aenv"
	// Description is a short, human-readable summary of the project used in
	// help output and documentation.
	Description = "Static environment compositor"
)

// AuthorInfo represents an individual author's name and email address.
type AuthorInfo struct {
	// Name is the author's preferred name or handle.
	Name string
	// Email is the author's contact email address.
	Email string
}

// Author lists the primary author(s) of the project for display in metadata.
//
//nolint:gochecknoglobals
var Author = []AuthorInfo{
	{"ardnew", "andrew@ardnew.com"},
}
