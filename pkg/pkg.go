package pkg

const (
	// Name is the canonical command and module identifier used across the
	// project. For example, it appears in help text and default config paths.
	Name = "aenv"
	// Description is a short, human-readable summary of the project used in
	// help output and documentation.
	Description = "Composite environment generator"
	// ProjectURL is the canonical project homepage.
	ProjectURL = "https://github.com/ardnew/aenv"
	// RepoURL is the canonical source repository URL.
	RepoURL = "https://github.com/ardnew/aenv"
)

// AuthorInfo represents an individual author's name and email address.
type AuthorInfo struct {
	// Name is the author's preferred name or handle.
	Name string
	// Email is the author's contact email address.
	Email string
}

// Author lists the primary author(s) of the project for display in metadata.
var Author = []AuthorInfo{
	{"ardnew", "andrew@ardnew.com"},
}
