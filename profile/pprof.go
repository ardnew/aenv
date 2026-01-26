//go:build pprof

package profile

import (
	"maps"
	"slices"
	"sync"

	"github.com/pkg/profile"

	_ "net/http/pprof" // register HTTP handlers
)

// Modes returns the list of supported profiling modes when built with the
// pprof build tag. The special mode "quiet" is omitted from the list.
var Modes = sync.OnceValue(
	func() []string {
		m := maps.Clone(mode)
		delete(m, "quiet")

		return slices.Sorted(maps.Keys(m))
	},
)

var mode = map[string]func(*profile.Profile){
	"block":     profile.BlockProfile,
	"cpu":       profile.CPUProfile,
	"clock":     profile.ClockProfile,
	"goroutine": profile.GoroutineProfile,
	"mem":       profile.MemProfile,
	"allocs":    profile.MemProfileAllocs,
	"heap":      profile.MemProfileHeap,
	"mutex":     profile.MutexProfile,
	"thread":    profile.ThreadcreationProfile,
	"trace":     profile.TraceProfile,
	"quiet":     profile.Quiet,
}

type control struct {
	mode []func(*profile.Profile)
}

func start(mode, path string, quiet bool) interface{ Stop() } {
	c := newControl(withMode(mode))

	if len(c.mode) == 0 {
		return ignore{}
	}

	return profile.Start(
		apply(c, withPath(path), withQuiet(quiet)).mode...,
	)
}

func withMode(m string) Option {
	return func(c control) control {
		if fn, ok := mode[m]; ok {
			c.mode = append(c.mode, fn)
		}

		return c
	}
}

func withPath(p string) Option {
	return func(c control) control {
		if p != "" {
			c.mode = append(c.mode, profile.ProfilePath(p))
		}

		return c
	}
}

func withQuiet(v bool) Option {
	return func(c control) control {
		if v {
			c.mode = append(c.mode, profile.Quiet)
		}

		return c
	}
}
