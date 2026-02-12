package pkg

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	expected := "aenv"
	if Name != expected {
		t.Errorf("Expected Name to be %q, got %q", expected, Name)
	}
}

func TestDescription(t *testing.T) {
	expected := "Static environment generator"
	if Description != expected {
		t.Errorf("Expected Description to be %q, got %q", expected, Description)
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
	buf, err := os.ReadFile(filepath.Join(repoRoot(t), "pkg", "VERSION"))
	if err != nil {
		t.Fatalf("Failed to read VERSION file: %v", err)
	}

	if content := strings.TrimSpace(string(buf)); Version != content {
		t.Errorf("Expected Version to be %q, got %q", content, Version)
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
