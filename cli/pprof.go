//go:build pprof

package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/profile"
)

const (
	pprofHelpGroup = "pprof"
	pprofHelpTitle = "Profiling (" + pprofHelpGroup + ")"
)

const defaultPprofAddr = "localhost:6060"

type pprofConfig struct {
	Mode string `default:"cpu"         enum:"${pprofModeEnum}" help:"Enable profiling (${enum})"                                placeholder:"MODE"        short:"m"`
	Dir  string `default:"${pprofDir}"                         help:"Profile output directory (${pprofDir})"                    placeholder:"PATH"                  type:"path"`
	HTTP bool   `default:"false"                               help:"Launch pprof HTTP server (default addr: ${pprofAddr})"                               short:"H"             negatable:""`
	Addr string `                                              help:"Override pprof HTTP listen address (implies --pprof-http)" placeholder:"[ADDR]:PORT"`
}

func (pprofConfig) vars() kong.Vars {
	return kong.Vars{
		"pprofModeEnum": strings.Join(slices.Sorted(profile.Modes()), ","),
		"pprofDir":      filepath.Join(cacheDir(), profile.Tag),
		"pprofAddr":     defaultPprofAddr,
	}
}

func (pprofConfig) group() kong.Group {
	var group kong.Group

	group.Key = pprofHelpGroup
	group.Title = pprofHelpTitle

	return group
}

// start starts profiling if configured.
//
// When a profiling mode is set, start begins writing profile data to
// [pprofConfig.Dir]. If [pprofConfig.HTTP] is set or [pprofConfig.Addr]
// is non-empty, an HTTP server is launched serving the handlers registered
// by [net/http/pprof]. Both are torn down when the returned stop function
// is called.
func (f pprofConfig) start(ctx context.Context) (stop func()) {
	if f.Mode == "" {
		return func() {}
	}

	var shutdownServer func()

	// Resolve the HTTP listen address. --addr implies --http.
	addr := f.Addr
	if addr == "" && f.HTTP {
		addr = defaultPprofAddr
	}

	// Optionally start net/http/pprof HTTP server so live profiling
	// endpoints are reachable at /debug/pprof/ while the application runs.
	if addr != "" {
		hctx, hcancel := context.WithCancelCause(ctx)

		server := new(http.Server)
		server.Addr = addr
		server.Handler = http.DefaultServeMux

		go func() {
			log.DebugContext(ctx, "pprof http start",
				slog.String("addr", addr),
			)

			err := server.ListenAndServe()

			if !errors.Is(err, http.ErrServerClosed) {
				log.ErrorContext(ctx, "pprof http server",
					slog.Any("error", err),
				)
				hcancel(err)
			}
		}()

		shutdownServer = func() { hcancel(server.Shutdown(hctx)) }
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

		if shutdownServer != nil {
			shutdownServer()
		}
	}
}
