package cli

import "fmt"

type (
	signed interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64
	}
	unsigned interface {
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
	}

	coordinate interface{ signed | unsigned }
)

type (
	point[T coordinate] struct{ x, y T }
	rect[T coordinate]  struct{ min, max point[T] }

	span[T coordinate]   struct{ min, max T }
	bounds[T coordinate] struct{ x, y span[T] }
)

func makeMaxBounds[T coordinate](x, y T) bounds[T] {
	return bounds[T]{
		x: span[T]{max: x},
		y: span[T]{max: y},
	}
}

func withMaxBounds[T coordinate](x, y T) option[bounds[T]] {
	return func(b *bounds[T]) {
		b.x.max = min(b.x.max, x)
		b.y.max = min(b.y.max, y)
	}
}
func (p point[T]) String() string  { return fmt.Sprintf("{x:%d,y:%d}", p.x, p.y) }
func (r rect[T]) String() string   { return fmt.Sprintf("{min:%s,max:%s}", r.min, r.max) }
func (s span[T]) String() string   { return fmt.Sprintf("{min:%d,max:%d}", s.min, s.max) }
func (b bounds[T]) String() string { return fmt.Sprintf("{x:%s,y:%s}", b.x, b.y) }
