package cli

type option[T any] func(*T)

func wrap[T any](t T, opts ...option[T]) T {
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

func withOptions[T any](opts ...option[T]) option[T] {
	return func(t *T) {
		for _, opt := range opts {
			opt(t)
		}
	}
}
