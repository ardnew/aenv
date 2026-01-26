//go:build pprof

package cli

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/profile"
)

type pprof struct {
	Mode string `default:""            enum:",${pprofModes}" help:"Enable profiling."         placeholder:"${enum}" short:"p"`
	Dir  string `default:"${pprofDir}"                       help:"Profile output directory."                                 type:"path"`
}

func (pprof) vars() kong.Vars {
	return kong.Vars{
		"pprofModes": strings.Join(profile.Modes(), ","),
		"pprofDir":   filepath.Join(cacheDir(), profile.Tag),
	}
}

func (pprof) group() kong.Group {
	var group kong.Group

	group.Key = "pprof"
	group.Title = "Profiling (pprof)"

	return group
}

// start starts profiling if configured.
func (f pprof) start(ctx context.Context) (stop func()) {
	if f.Mode == "" {
		return func() {}
	}

	log.DebugContext(ctx, "pprof start",
		slog.String("mode", f.Mode),
		slog.String("dir", f.Dir),
	)

	// Create base config and apply options
	var cfg profile.Config = func() (string, string, bool) {
		return "", "", false
	}

	cfg = profile.WithMode(f.Mode)(cfg)
	cfg = profile.WithPath(f.Dir)(cfg)
	cfg = profile.WithQuiet(true)(cfg)
	profiler := cfg.Start()

	return func() {
		log.DebugContext(ctx, "pprof stop",
			slog.String("mode", f.Mode),
			slog.String("dir", f.Dir),
		)
		profiler.Stop()
	}
}
