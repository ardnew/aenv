package pkg

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Prefix returns the base prefix string used to construct the path to the
// configuration directory and the prefix for environment variable identifiers.
//
// By default, Prefix is the base name of the executable file unless it matches
// one of the following substitution rules:
//   - "__debug_bin" (default output of the dlv debugger): replaced with cmd
//   - "^\.+" (dot-prefixed names): remove the dot prefix
//
//nolint:gochecknoglobals
var Prefix = sync.OnceValue(
	func() string {
		id := os.Args[0]
		exe, err := os.Executable()
		if err == nil {
			id = exe
		}

		ext := filepath.Ext(filepath.Base(id))
		id = strings.TrimSuffix(filepath.Base(id), ext)

		for rex, rep := range map[*regexp.Regexp]string{
			regexp.MustCompile(`^__debug_bin\d+$`): Name, // default output from dlv
			regexp.MustCompile(`^\.+`):             "",   // remove leading dot(s)
		} {
			id = rex.ReplaceAllString(id, rep)
		}

		return id
	},
)

// ConfigDir returns the configuration directory path.
//
//nolint:gochecknoglobals
var ConfigDir = sync.OnceValue(
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

		return filepath.Join(dir, Prefix())
	},
)

// CacheDir returns the cache directory path used for transient files.
//
//nolint:gochecknoglobals
var CacheDir = sync.OnceValue(
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

		return filepath.Join(dir, Prefix())
	},
)
