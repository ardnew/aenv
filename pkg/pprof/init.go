package pprof

// Profiler configures and initializes the profiler.
type Profiler struct {
	Mode  string
	Path  string
	Quiet bool
}

// Start initializes the profiler and returns an interface for stopping it.
//
// Mode specifies the profiler mode to use, and path specifies the default
// output directory where profiling data will be written.
//
// If build tag pprof or p.Mode are unset, then Start returns a no-op
// implementation.
// Both Start and Stop are always safely callable.
func (p Profiler) Start() interface{ Stop() } {
	// If no mode is specified, do nothing.
	if p.Mode == "" {
		return ignore{}
	}

	return start(p.Mode, p.Path, p.Quiet)
}

// WithMode returns a functional option for setting a profiler's mode.
func WithMode(mode string) func(Profiler) Profiler {
	return func(p Profiler) Profiler {
		p.Mode = mode

		return p
	}
}

// WithPath returns a functional option for setting a profiler's output path.
func WithPath(path string) func(Profiler) Profiler {
	return func(p Profiler) Profiler {
		p.Path = path

		return p
	}
}

// WithQuiet returns a functional option for setting a profiler's quiet flag.
func WithQuiet(quiet bool) func(Profiler) Profiler {
	return func(p Profiler) Profiler {
		p.Quiet = quiet

		return p
	}
}

type ignore struct{}

func (ignore) Stop() {}
