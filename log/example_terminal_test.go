package log

import (
"fmt"
"io"
)

func ExampleIsTerminal_terminalWriter() {
// A writer implementing TerminalWriter is treated as a terminal.
var w exampleTerminalWriter
fmt.Println(IsTerminal(&w))
// Output:
// true
}

func ExampleIsTerminal_nonTerminal() {
// io.Discard has no Fd and does not implement TerminalWriter.
fmt.Println(IsTerminal(io.Discard))
// Output:
// false
}
