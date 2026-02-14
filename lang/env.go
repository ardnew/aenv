package lang

// This file defines the built-in evaluation environment available to all
// expr-lang expressions. The environment is lazily initialized once per
// process via envCache and cloned on every access so callers may mutate
// the returned map without affecting the shared cache.
//
// Built-in names can be shadowed by user-defined namespace identifiers.

import (
	"bufio"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/ardnew/mung"
)

// Private singleton cache.
//
//nolint:gochecknoglobals
var (
	envCacheOnce sync.Once
	envCache     map[string]any
)

// makeEnvCache returns a clone of the lazily-initialized, process-scoped
// environment containing built-in variables and functions. The returned map
// can be safely mutated by the caller without affecting the shared cache.
func makeEnvCache() map[string]any {
	envCacheOnce.Do(func() {
		envCache = map[string]any{
			// System information (struct/string values).
			"target":   getTarget(),
			"platform": getPlatform(),
			"hostname": getHostname(),
			"user":     getUser(),
			"shell":    getShell(),

			// Working directory.
			"cwd": getCwd,

			// Filesystem functions.
			"file": map[string]any{
				"exists":    fileExists,
				"isDir":     fileIsDir,
				"isRegular": fileIsRegular,
				"isSymlink": fileIsSymlink,
			},

			// Path manipulation functions.
			"path": map[string]any{
				"abs": pathAbs,
				"cat": pathCat,
				"rel": pathRel,
			},

			// PATH-like string manipulation via mung.
			"mung": map[string]any{
				"prefix":   mungPrefix,
				"prefixif": mungPrefixIf,
			},
		}
	})

	return maps.Clone(envCache)
}

// BuiltinEnvCache returns a copy of the built-in environment cache.
// This is useful for reflection-based signature introspection.
func BuiltinEnvCache() map[string]any {
	return makeEnvCache()
}

// BuiltinEnvKeys returns the top-level keys in the built-in environment.
// This is useful for code completion and introspection.
func BuiltinEnvKeys() []string {
	env := makeEnvCache()
	keys := make([]string, 0, len(env)+1)

	for k := range env {
		keys = append(keys, k)
	}

	// Add "env" which is populated at runtime with process environment
	keys = append(keys, "env")

	return keys
}

// BuiltinEnvLookup looks up a dot-separated path in the built-in environment
// and returns the keys of any map found at that path. Returns nil if the path
// doesn't exist or doesn't point to a map.
//
// Special case: "env" returns environment variable names from os.Environ().
func BuiltinEnvLookup(path string) []string {
	if path == "" {
		return BuiltinEnvKeys()
	}

	// Special handling for "env" namespace (process environment)
	if path == "env" {
		envMap := buildProcessEnvMap(nil)

		keys := make([]string, 0, len(envMap))
		for k := range envMap {
			keys = append(keys, k)
		}

		return keys
	}

	env := makeEnvCache()
	segments := strings.Split(path, ".")

	var current any = env

	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}

		current, ok = m[seg]
		if !ok {
			return nil
		}
	}

	// If we found a map, return its keys
	if m, ok := current.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}

		return keys
	}

	return nil
}

// ---------------------------------------------------------------------------
// System information helpers
// ---------------------------------------------------------------------------

// target contains string identifiers for a target operating system and
// instruction set architecture.
//
// Leaving the conventions unspecified allows this type to be used
// in a variety of contexts.
type target struct {
	OS   string
	Arch string
}

// getTarget returns the host target using GNU GCC/LLVM naming conventions.
func getTarget() target {
	t := getPlatform()

	switch t.Arch {
	case "386":
		t.Arch = "i386"
	case "amd64":
		t.Arch = "x86_64"
	case "arm":
		arm, ok := os.LookupEnv("GOARM")
		if ok {
			arm, _, _ = strings.Cut(arm, ",")
			switch strings.TrimSpace(arm) {
			case "5", "6", "7":
				t.Arch = "armv" + arm
			}
		}
	case "arm64":
		if t.OS != "darwin" {
			t.Arch = "aarch64"
		}
	case "mipsle":
		t.Arch = "mipsel"
	}

	return t
}

// getPlatform returns the host target using Go conventions.
//
// [Go conventions]:
// https://cs.opensource.google/go/go/+/master:src/cmd/dist/build.go
func getPlatform() target {
	var (
		o, a string
		ok   bool
	)

	if o, ok = os.LookupEnv("GOHOSTOS"); !ok {
		if o, ok = os.LookupEnv("GOOS"); !ok {
			o = runtime.GOOS
		}
	}

	if a, ok = os.LookupEnv("GOHOSTARCH"); !ok {
		if a, ok = os.LookupEnv("GOARCH"); !ok {
			a = runtime.GOARCH
		}
	}

	return target{
		OS:   o,
		Arch: a,
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}

	return hostname
}

func getUser() *user.User {
	u, err := user.Current()
	if err != nil {
		return nil
	}

	return u
}

func getShell() string {
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		return shell
	}

	u := getUser()
	if u == nil || u.Username == "" {
		return ""
	}

	f, err := os.Open("/etc/passwd")
	if err != nil {
		return ""
	}

	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		l := s.Text()

		e := strings.Split(l, ":")
		if len(e) > 6 && e[0] == u.Username {
			return e[6]
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// Working directory
// ---------------------------------------------------------------------------

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return pathAbs(".")
	}

	return cwd
}

// ---------------------------------------------------------------------------
// Filesystem functions
// ---------------------------------------------------------------------------

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}

func fileIsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func fileIsRegular(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular()
}

func fileIsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeSymlink != 0
}

// ---------------------------------------------------------------------------
// Path manipulation functions
// ---------------------------------------------------------------------------

func pathAbs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return p
}

func pathCat(elem ...string) string {
	return filepath.Join(elem...)
}

func pathRel(from, to string) string {
	p, err := filepath.Rel(pathAbs(from), pathAbs(to))
	if err != nil {
		return pathCat(from, to)
	}

	return p
}

// ---------------------------------------------------------------------------
// PATH-like string manipulation (mung)
// ---------------------------------------------------------------------------

func mungPrefix(key string, prefix ...string) string {
	return mung.Make(
		mung.WithSubjectItems(key),
		mung.WithDelim(string(os.PathListSeparator)),
		mung.WithPrefixItems(prefix...),
	).String()
}

func mungPrefixIf(
	key string,
	predicate func(string) bool,
	prefix ...string,
) string {
	return mung.Make(
		mung.WithSubjectItems(key),
		mung.WithDelim(string(os.PathListSeparator)),
		mung.WithPrefixItems(prefix...),
		mung.WithFilter(predicate),
	).String()
}

// ---------------------------------------------------------------------------
// Environment variable function
// ---------------------------------------------------------------------------

// buildProcessEnvMap converts a "KEY=VALUE" string slice to a map.
// If envList is nil, os.Environ() is used.
func buildProcessEnvMap(envList []string, keyVal ...string) map[string]string {
	envList = append(envList, keyVal...)
	if len(envList) == 0 {
		envList = os.Environ()
	}

	result := make(map[string]string, len(envList))

	for _, entry := range envList {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}

	return result
}

// envFunc returns the built-in env() function that provides
// process environment access to expr programs.
func envFunc(processEnv map[string]string) func(string) string {
	return func(key string) string {
		return processEnv[key]
	}
}
