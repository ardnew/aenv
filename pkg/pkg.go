package pkg

const (
	// Name is the command and module name.
	Name = "aenv"
	// Description is a one-line project summary.
	Description = "Composite environment generator"
	// ProjectURL is the project homepage.
	ProjectURL = "https://github.com/ardnew/aenv"
	// RepoURL is the source repository.
	RepoURL = "https://github.com/ardnew/aenv"
)

// AuthorInfo is an author's name and email.
type AuthorInfo struct {
	// Name is the author's name.
	Name string
	// Email is the author's email.
	Email string
}

// Author lists the project authors.
var Author = []AuthorInfo{
	{"ardnew", "andrew@ardnew.com"},
}
