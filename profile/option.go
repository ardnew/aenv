//go:build pprof

package profile

// Option applies a configuration option to control.
type Option func(control) control

// apply applies multiple options to a control.
func apply(c control, opts ...Option) control {
	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// newControl creates a new control with the provided options.
func newControl(opts ...Option) control {
	var c control

	return apply(c, opts...)
}
