package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ardnew/aenv/pkg"
)

// baseConfig is the base name of the configuration file and namespace.
const baseConfig = "config"

// DefaultDirMode is the default permission mode for created directories.
var defaultDirMode os.FileMode = 0o700

// basePrefix returns the base prefix string used to construct the path to the
// configuration directory and the prefix for environment variable identifiers.
//
// By default, basePrefix is the base name of the executable file unless it
// matches one of the following substitution rules:
//   - "__debug_bin" (default output of the dlv debugger): replaced with cmd
//   - "^\.+" (dot-prefixed names): remove the dot prefix
var basePrefix = sync.OnceValue(
	func() string {
		id := os.Args[0]
		exe, err := os.Executable()
		if err == nil {
			id = exe
		}

		ext := filepath.Ext(filepath.Base(id))
		id = strings.TrimSuffix(filepath.Base(id), ext)

		for rex, rep := range map[*regexp.Regexp]string{
			regexp.MustCompile(`^__debug_bin\d+$`): pkg.Name, // dlv default output
			regexp.MustCompile(`^\.+`):             "",       // remove leading dot(s)
		} {
			id = rex.ReplaceAllString(id, rep)
		}

		return id
	},
)

// configDir returns the configuration directory path.
var configDir = sync.OnceValue(
	func() string {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir, err = os.UserHomeDir()
			if err == nil {
				dir = filepath.Join(dir, ".config")
			} else {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					dir = "."
				}
			}
		}

		return filepath.Join(dir, basePrefix())
	},
)

// cacheDir returns the cache directory path used for transient files.
var cacheDir = sync.OnceValue(
	func() string {
		dir, err := os.UserCacheDir()
		if err != nil {
			dir, err = os.UserHomeDir()
			if err == nil {
				dir = filepath.Join(dir, ".cache")
			} else {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					dir = "."
				}
			}
		}

		return filepath.Join(dir, basePrefix())
	},
)

// configPath returns the absolute path to a file or directory formed by joining
// the global configuration directory path with the given path elements.
//
// If no elements are given, it is equivalent to calling [configDir].
func configPath(elem ...string) string {
	return filepath.Join(append([]string{configDir()}, elem...)...)
}

// mkdirAllRequired creates all required runtime directories.
func mkdirAllRequired() error {
	// Create base config directory
	err := os.MkdirAll(configDir(), defaultDirMode)
	if err != nil {
		return err
	}

	// Create base cache directory
	err = os.MkdirAll(cacheDir(), defaultDirMode)
	if err != nil {
		return err
	}

	return nil
}
