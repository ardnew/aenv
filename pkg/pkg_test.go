package pkg

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	version, err := os.ReadFile(filepath.Join("..", "VERSION"))
	if err != nil {
		panic(err)
	}
	license, err := os.ReadFile(filepath.Join("..", "LICENSE"))
	if err != nil {
		panic(err)
	}
	v, l := string(version), string(license)
	Meta.Version = v
	Meta.License = l
	os.Exit(m.Run())
}

func TestName(t *testing.T) {
	expected := "aenv"
	if Name != expected {
		t.Errorf("Expected Name to be %q, got %q", expected, Name)
	}
}

func TestDescription(t *testing.T) {
	expected := "Composite environment generator"
	if Description != expected {
		t.Errorf("Expected Description to be %q, got %q", expected, Description)
	}
}

func TestURLs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "project", raw: ProjectURL},
		{name: "repo", raw: RepoURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.raw == "" {
				t.Fatal("URL must not be empty")
			}
			parsed, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("url.Parse(%q) error = %v", tt.raw, err)
			}
			if parsed.Scheme != "https" || parsed.Host == "" {
				t.Fatalf("URL = %q, want absolute https URL", tt.raw)
			}
		})
	}
	if RepoURL != "https://github.com/ardnew/aenv" {
		t.Fatalf("RepoURL = %q, want github repository URL", RepoURL)
	}
}

func repoRoot(t *testing.T) string {
	cmd := exec.CommandContext(t.Context(), "git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get root of git repo: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestVersion(t *testing.T) {
	// Version is embedded from VERSION file, so it should not be empty.
	buf, err := os.ReadFile(filepath.Join(repoRoot(t), "VERSION"))
	if err != nil {
		t.Fatalf("Failed to read VERSION file: %v", err)
	}

	got := strings.TrimSpace(Meta.Version)
	if content := strings.TrimSpace(string(buf)); got != content {
		t.Errorf("Expected Version to be %q, got %q", content, got)
	}
}

func TestAuthor(t *testing.T) {
	if len(Author) == 0 {
		t.Error("Expected Author to have at least one entry")
	}

	// Test if a known author is present
	if len(Author) > 0 {
		expectedName := "ardnew"
		expectedEmail := "andrew@ardnew.com"

		if !slices.ContainsFunc(Author, func(a AuthorInfo) bool {
			return a.Name == expectedName && a.Email == expectedEmail
		}) {
			t.Errorf("Expected Author to contain %q, %q", expectedName, expectedEmail)
		}
	}
}

func TestAuthorStruct(t *testing.T) {
	// Test that Author slice has the expected structure
	for i, author := range Author {
		if author.Name == "" && author.Email == "" {
			t.Errorf("Author[%d] must define at least Name or Email", i)
		}
	}
}
