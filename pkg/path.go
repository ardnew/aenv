package pkg

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CachePath joins elem onto the cache directory and returns an absolute path.
func CachePath(elem ...string) string {
	return filepath.Join(append([]string{cacheDir()}, elem...)...)
}

// ConfigPath joins elem onto the config directory and returns an absolute path.
func ConfigPath(elem ...string) string {
	return filepath.Join(append([]string{configDir()}, elem...)...)
}

// EntryPath returns the entry-point file path and true, or "" and false if
// none is found.
func EntryPath() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	home, _ := os.UserHomeDir()

	cfgDirs := append([]string{configDir()}, xdgConfigDirs()...)
	return resolveEntryPath(cwd, home, cfgDirs)
}

const (
	localEntryFile  = "." + Name + "rc" // file name during CWD traversal
	globalEntryFile = Name + "rc"       // file name in XDG config directories
)

// rootPrefix returns the physical root prefix of the system and a logical root
// prefix specified by the environment variable [Name]_PREFIX, if set.
//
// The physical root prefix is "/" on Unix and "<Drive>:" on Windows, where the
// drive is determined by the current working directory.
//
// The logical root prefix is used for resolving system paths such as
// "[rootPrefix]/etc", "[rootPrefix]/tmp", and others, and defaults to the
// physical root prefix if the environment variable is not set.
var rootPrefix = sync.OnceValues(
	func() (phy, log string) {
		// filepath.VolumeName requires an absolute path to extract the drive letter
		// on Windows; a bare "." is relative and always returns "".
		cwd, _ := os.Getwd()
		phy = filepath.VolumeName(cwd) + string(filepath.Separator)
		log = os.Getenv(strings.ToUpper(Name) + "_PREFIX")
		if log == "" {
			log = phy
		}
		return phy, log

	},
)

var cacheDir = sync.OnceValue(
	func() string {
		dir, err := os.UserCacheDir()
		if err != nil {
			dir, err = os.UserHomeDir()
			if err == nil {
				dir = filepath.Join(dir, ".cache")
			} else {
				dir = os.TempDir()
			}
		}

		return filepath.Join(dir, Name)
	},
)

var configDir = sync.OnceValue(
	func() string {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir, err = os.UserHomeDir()
			if err == nil {
				dir = filepath.Join(dir, ".config")
			} else {
				dir = os.TempDir()
			}
		}

		return filepath.Join(dir, Name)
	},
)

var xdgConfigDirs = sync.OnceValue(
	func() []string {
		if val := os.Getenv("XDG_CONFIG_DIRS"); val != "" {
			var dirs []string
			for _, dir := range strings.Split(val, string(filepath.ListSeparator)) {
				if dir != "" {
					dirs = append(dirs, filepath.Join(dir, Name))
				}
			}
			return dirs
		}

		_, log := rootPrefix()
		return []string{filepath.Join(log, "etc", "xdg", Name)}
	},
)

func resolveEntryPath(cwd, home string, cfgDirs []string) (string, bool) {
	phy, log := rootPrefix()

	homeSweetHome := false
	dir := cwd
	for {
		if path := filepath.Join(dir, localEntryFile); isRegularFile(path) {
			return path, true
		}

		if dir == home {
			homeSweetHome = true
		}
		if dir == home || dir == log || dir == phy {
			break
		}

		dir = filepath.Dir(dir)
	}

	if !homeSweetHome && home != "" {
		if path := filepath.Join(home, localEntryFile); isRegularFile(path) {
			return path, true
		}
	}

	for _, dir := range cfgDirs {
		if dir == "" {
			continue
		}
		if path := filepath.Join(dir, globalEntryFile); isRegularFile(path) {
			return path, true
		}
	}

	return "", false
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
