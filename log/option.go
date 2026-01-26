package log

// Option applies a configuration option to config.
type Option func(config) config

// apply applies multiple options to a config.
func apply(cfg config, opts ...Option) config {
	for _, opt := range opts {
		cfg = opt(cfg)
	}

	return cfg
}
