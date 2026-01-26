package profile

// Config functions return all supported pprof configuration parameters.
type Config func() (mode, path string, quiet bool)

// Start initializes the profiler and returns an interface for stopping it.
//
// Mode specifies the profiler mode to use, and path specifies the default
// output directory where profiling data will be written.
//
// If build tag pprof or p.Mode are unset, then Start returns a no-op
// implementation.
// Both Start and Stop are always safely callable.
func (c Config) Start() interface{ Stop() } {
	mode, path, quiet := c()

	if mode == "" {
		return ignore{}
	}

	return start(mode, path, quiet)
}

// WithMode returns a functional option for setting a profiler's mode.
func WithMode(mode string) func(Config) Config {
	return func(c Config) Config {
		_, path, quiet := c()

		return func() (string, string, bool) {
			return mode, path, quiet
		}
	}
}

// WithPath returns a functional option for setting a profiler's output path.
func WithPath(path string) func(Config) Config {
	return func(c Config) Config {
		mode, _, quiet := c()

		return func() (string, string, bool) {
			return mode, path, quiet
		}
	}
}

// WithQuiet returns a functional option for setting a profiler's quiet flag.
func WithQuiet(quiet bool) func(Config) Config {
	return func(c Config) Config {
		mode, path, _ := c()

		return func() (string, string, bool) {
			return mode, path, quiet
		}
	}
}

type ignore struct{}

func (ignore) Stop() {}
