package pkg

import "iter"

// Unused is a no-op function that can be used to
// suppress unused variable warnings for one or more variables.
//
// This is useful when you want to keep variable names somewhere
// even if they are not currently used in the code.
// (signatures of exported functions, interface implementations, etc.)
func Unused(...any) {}

// TypeCast is function that converts a value of type T to type U.
type TypeCast[T, U any] func(T) U

// AnyValues returns the given values of type T as a sequence of any.
func AnyValues[T any](v ...T) iter.Seq[any] {
	var fn TypeCast[T, any] = func(v T) any { return v }

	return fn.Values(v...)
}

// Values returns an iterator over the given values, casting each value
// from type T to type U using the TypeCast receiver.
func (c TypeCast[T, U]) Values(v ...T) iter.Seq[U] {
	return func(yield func(U) bool) {
		for _, x := range v {
			if !yield(c(x)) {
				return
			}
		}
	}
}
