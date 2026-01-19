package pkg

// Option is a functional option that returns its argument with
// modifications applied.
type Option[T any] func(T) T

// Make returns a new object of type T with the given options applied.
//
//nolint:ireturn
func Make[T any](opts ...Option[T]) (t T) {
	return Wrap(t, opts...)
}

// Wrap returns t after applying the given options.
//
//nolint:ireturn
func Wrap[T any](t T, opts ...Option[T]) T {
	for _, o := range opts {
		t = o(t)
	}

	return t
}

// WithIdentity returns an option that returns its argument unmodified.
func WithIdentity[T any]() Option[T] {
	return func(t T) T {
		return t
	}
}
