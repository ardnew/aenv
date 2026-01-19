//go:build pprof

package cli

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/envcomp/pkg"
	_log "github.com/ardnew/envcomp/pkg/log"
	_pprof "github.com/ardnew/envcomp/pkg/pprof"
)

type pprof struct {
	Mode string `default:""            enum:",${pprofModes}" help:"Enable profiling."         placeholder:"${enum}" short:"p"`
	Dir  string `default:"${pprofDir}"                       help:"Profile output directory."                                 type:"path"`
}

func (pprof) vars() kong.Vars {
	return kong.Vars{
		"pprofModes": strings.Join(_pprof.Modes(), ","),
		"pprofDir":   filepath.Join(pkg.CacheDir(), _pprof.Tag),
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

	_log.DebugContext(ctx, "pprof start",
		slog.String("mode", f.Mode),
		slog.String("dir", f.Dir),
	)

	profiler := pkg.Make(
		_pprof.WithMode(f.Mode),
		_pprof.WithPath(f.Dir),
		_pprof.WithQuiet(true),
	).Start()

	return func() {
		_log.DebugContext(ctx, "pprof stop",
			slog.String("mode", f.Mode),
			slog.String("dir", f.Dir),
		)
		profiler.Stop()
	}
}
