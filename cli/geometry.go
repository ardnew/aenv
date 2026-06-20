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
	span[T coordinate]   struct{ min, max T }
	bounds[T coordinate] struct{ x, y span[T] }
)

func makeMaxBounds[T coordinate](x, y T) bounds[T] {
	return bounds[T]{
		x: span[T]{max: x},
		y: span[T]{max: y},
	}
}

func (s span[T]) String() string   { return fmt.Sprintf("{min:%d,max:%d}", s.min, s.max) }
func (b bounds[T]) String() string { return fmt.Sprintf("{x:%s,y:%s}", b.x, b.y) }
