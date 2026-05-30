package exit

import "fmt"

// code is a minimal Coder for examples.
type code int

func (c code) ExitCode() int { return int(c) }

func ExampleIsError() {
fmt.Println(IsError(code(Usage)))
fmt.Println(IsError(code(OK)))
// Output:
// true
// false
}
