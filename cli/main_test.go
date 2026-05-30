package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ardnew/aenv/pkg"
)

func TestMain(m *testing.M) {
	root := filepath.Join("..")
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		panic(err)
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		panic(err)
	}
	v, l := string(version), string(license)
	pkg.Meta.Version = v
	pkg.Meta.License = l
	os.Exit(m.Run())
}
