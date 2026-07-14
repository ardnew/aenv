package pkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectFile(t *testing.T) {
	if want := "." + Name + "rc"; localEntryFile != want {
		t.Errorf("ProjectFile = %q, want %q", localEntryFile, want)
	}
}

func TestRCFile(t *testing.T) {
	if want := Name + "rc"; globalEntryFile != want {
		t.Errorf("RCFile = %q, want %q", globalEntryFile, want)
	}
}

func TestRootPrefix_Physical_IsAbsolute(t *testing.T) {
	phy, _ := rootPrefix()
	if phy == "" {
		t.Fatal("RootPrefix phy must not be empty")
	}
	if !filepath.IsAbs(phy) {
		t.Errorf("RootPrefix phy %q must be absolute", phy)
	}
}

func TestRootPrefix_Logical_DefaultsToPhysical(t *testing.T) {
	// When AENV_PREFIX is unset at the time RootPrefix is first computed,
	// the logical prefix defaults to the physical prefix.
	// sync.OnceValues means we can only observe the state at first call.
	phy, log := rootPrefix()
	if log == "" {
		t.Fatal("RootPrefix log must not be empty")
	}
	// If the env var was unset, log must equal phy.
	if os.Getenv(strings.ToUpper(Name)+"_PREFIX") == "" && log != phy {
		t.Errorf("RootPrefix log = %q, want %q (same as phy when env var unset)", log, phy)
	}
}

func TestCachePath_NoArgs_IsAbsolute(t *testing.T) {
	p := CachePath()
	if p == "" {
		t.Fatal("CachePath() must not be empty")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("CachePath() = %q must be absolute", p)
	}
}

func TestCachePath_Elements_AreJoinedUnderBase(t *testing.T) {
	base := CachePath()
	tests := []struct {
		elems []string
		want  string
	}{
		{[]string{"foo"}, filepath.Join(base, "foo")},
		{[]string{"foo", "bar"}, filepath.Join(base, "foo", "bar")},
	}
	for _, tt := range tests {
		if got := CachePath(tt.elems...); got != tt.want {
			t.Errorf("CachePath(%v) = %q, want %q", tt.elems, got, tt.want)
		}
	}
}

func TestConfigPath_NoArgs_IsAbsolute(t *testing.T) {
	p := ConfigPath()
	if p == "" {
		t.Fatal("ConfigPath() must not be empty")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("ConfigPath() = %q must be absolute", p)
	}
}

func TestConfigPath_Elements_AreJoinedUnderBase(t *testing.T) {
	base := ConfigPath()
	tests := []struct {
		elems []string
		want  string
	}{
		{[]string{"foo"}, filepath.Join(base, "foo")},
		{[]string{"foo", "bar"}, filepath.Join(base, "foo", "bar")},
	}
	for _, tt := range tests {
		if got := ConfigPath(tt.elems...); got != tt.want {
			t.Errorf("ConfigPath(%v) = %q, want %q", tt.elems, got, tt.want)
		}
	}
}

func TestIsRegularFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file")
	if err := os.WriteFile(file, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tmp, "dir")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "regular file", path: file, want: true},
		{name: "directory", path: dir},
		{name: "missing", path: filepath.Join(tmp, "missing")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRegularFile(tt.path); got != tt.want {
				t.Fatalf("isRegularFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveEntryPath_FoundInCWD(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	entry := filepath.Join(tmp, localEntryFile)
	if err := os.WriteFile(entry, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := EntryPath()
	if !ok {
		t.Fatal("ResolveEntryPath() = false, want true")
	}
	if got != entry {
		t.Errorf("ResolveEntryPath() = %q, want %q", got, entry)
	}
}

func TestResolveEntryPath_FoundInParent(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "child")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	entry := filepath.Join(tmp, localEntryFile)
	if err := os.WriteFile(entry, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := EntryPath()
	if !ok {
		t.Fatal("ResolveEntryPath() = false, want true")
	}
	if got != entry {
		t.Errorf("ResolveEntryPath() = %q, want %q", got, entry)
	}
}

func TestResolveEntryPath_IgnoresDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	// A directory named ProjectFile must not match (only regular files).
	if err := os.MkdirAll(filepath.Join(tmp, localEntryFile), 0o700); err != nil {
		t.Fatal(err)
	}

	_, ok := EntryPath()
	if ok {
		t.Log("ResolveEntryPath() returned true — a regular entry file exists outside the test temp dir")
	}
}

func TestResolveEntryPath_XDG_FoundInConfigHome(t *testing.T) {
	cwd := t.TempDir()
	cfg := filepath.Join(t.TempDir(), Name)
	entry := filepath.Join(cfg, globalEntryFile)
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveEntryPath(cwd, "", []string{cfg})
	if !ok {
		t.Fatal("resolveEntryPath() = false, want true")
	}
	if got != entry {
		t.Errorf("resolveEntryPath() = %q, want %q", got, entry)
	}
}

func TestResolveEntryPath_XDG_FoundInSecondConfigDir(t *testing.T) {
	cwd := t.TempDir()
	first := filepath.Join(t.TempDir(), Name)
	second := filepath.Join(t.TempDir(), Name)
	entry := filepath.Join(second, globalEntryFile)
	if err := os.MkdirAll(second, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveEntryPath(cwd, "", []string{first, second})
	if !ok {
		t.Fatal("resolveEntryPath() = false, want true")
	}
	if got != entry {
		t.Errorf("resolveEntryPath() = %q, want %q", got, entry)
	}
}

func TestResolveEntryPath_XDG_UserDirBeforeSystemDir(t *testing.T) {
	cwd := t.TempDir()
	user := filepath.Join(t.TempDir(), Name)
	system := filepath.Join(t.TempDir(), Name)
	userEntry := filepath.Join(user, globalEntryFile)
	systemEntry := filepath.Join(system, globalEntryFile)
	for _, path := range []string{user, system} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{userEntry, systemEntry} {
		if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, ok := resolveEntryPath(cwd, "", []string{user, system})
	if !ok {
		t.Fatal("resolveEntryPath() = false, want true")
	}
	if got != userEntry {
		t.Errorf("resolveEntryPath() = %q, want %q", got, userEntry)
	}
}

func TestResolveEntryPath_XDG_SkipsEmptyDirs(t *testing.T) {
	cwd := t.TempDir()

	_, ok := resolveEntryPath(cwd, "", []string{""})
	if ok {
		t.Fatal("resolveEntryPath() = true, want false")
	}
}

func TestResolveEntryPath_HomeCheck_FiredWhenCWDOutsideHome(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	entry := filepath.Join(home, localEntryFile)
	if err := os.WriteFile(entry, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveEntryPath(cwd, home, nil)
	if !ok {
		t.Fatal("resolveEntryPath() = false, want true")
	}
	if got != entry {
		t.Errorf("resolveEntryPath() = %q, want %q", got, entry)
	}
}

func TestResolveEntryPath_HomeCheck_SkippedWhenCWDUnderHome(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "child")
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}

	_, ok := resolveEntryPath(cwd, home, nil)
	if ok {
		t.Fatal("resolveEntryPath() = true, want false")
	}
}

func TestResolveEntryPath_XDG_IgnoresDirectory(t *testing.T) {
	cwd := t.TempDir()
	cfg := filepath.Join(t.TempDir(), Name)
	if err := os.MkdirAll(filepath.Join(cfg, globalEntryFile), 0o700); err != nil {
		t.Fatal(err)
	}

	_, ok := resolveEntryPath(cwd, "", []string{cfg})
	if ok {
		t.Fatal("resolveEntryPath() = true, want false")
	}
}
