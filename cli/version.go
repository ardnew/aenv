package cli

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/pkg"
)

// Version is the version subcommand. All flags are mutually exclusive.
type Version struct {
	// Semantic prints the semantic version.
	Semantic bool `help:"Print only the semantic version." short:"s" xor:"version-output"`
	// URL prints the project URL.
	URL bool `help:"Print the project URL." xor:"version-output"`
	// Repo prints the repository URL.
	Repo bool `help:"Print the repository URL." xor:"version-output"`
	// License prints the license.
	License bool `help:"Print the license." xor:"version-output"`
}

func (v Version) String() string {
	version := strings.TrimSpace(pkg.Meta.Version)
	switch {
	case v.Semantic:
		return version
	case v.URL:
		return pkg.ProjectURL
	case v.Repo:
		return pkg.RepoURL
	case v.License:
		return pkg.Meta.License
	default:
		return fmt.Sprintf("%s version %s", pkg.Name, version)
	}
}

// Run executes the version subcommand.
func (v Version) Run(app *kong.Kong) error {
	_, err := fmt.Fprintln(app.Stdout, v.String())

	return withExitCode(err, exit.IO)
}
