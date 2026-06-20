package lang

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ardnew/aenv/log"
)

const parseErrorContextWidth = 48

type ParseError struct {
	Err error
	Pos Pos

	srcContext string
}

func MakeParseError(err error, pos Pos, r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)

	var iLine int
	for ; iLine < pos.line && scan.Scan(); iLine++ {
	}

	if serr := scan.Err(); serr != nil {
		const m = `failed to scan source for error reporting`
		log.Error(log.Attrs(
			"parse-error", err,
			"scan-error", serr,
		), m)
		panic(fmt.Errorf("%w: "+m+": %w", err, serr))
	}

	line := scan.Text()
	if pos.column-1 > len(line) {
		const m = `failed to index source line for error reporting`
		log.Error(log.Attrs(
			"parse-error", err,
			"source-line", line,
			"column-index", pos.column,
		), m)
		panic(fmt.Errorf("%w: "+m, err))
	}

	return &ParseError{
		Err:        err,
		Pos:        pos,
		srcContext: buildContext(line, pos, parseErrorContextWidth),
	}
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s at line %d, column %d",
		e.Err.Error(), e.Pos.line, e.Pos.column)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func (e *ParseError) Snippet() string {
	return e.srcContext
}

func buildContext(line string, pos Pos, width int) string {
	// Capture a span of the line around the error position, with a fixed width.
	var span = struct {
		beg, end           int
		truncBeg, truncEnd bool
	}{
		beg: pos.column - 1 - (width / 2),
		end: pos.column - 1 + (width / 2),
	}
	if span.beg < 0 {
		span.end += -span.beg
		span.beg = 0
	}
	if span.end > len(line) {
		span.beg -= span.end - len(line)
		span.end = len(line)
		if span.beg < 0 {
			span.beg = 0
		}
	}

	span.truncBeg = span.beg > 0
	span.truncEnd = span.end < len(line)

	// Convert to rune slice for simpler editing.
	runes := []rune(line)

	// Replace truncated text with ellipsis if it's not just whitespace.
	if span.truncBeg && strings.TrimSpace(string(runes[:span.beg])) != "" {
		runes[span.beg] = '…'
	}
	if span.truncEnd && strings.TrimSpace(string(runes[span.end:])) != "" {
		runes[span.end-1] = '…'
	}

	var sb strings.Builder
	sb.WriteString(string(runes[span.beg:span.end]))
	sb.WriteByte('\n')

	// Add a marker line with an arrow pointing to the error column.
	for i := span.beg; i < span.end; i++ {
		if i == pos.column-1 {
			sb.WriteRune('↑')
		} else {
			sb.WriteByte(' ')
		}
	}

	return sb.String()
}

//123456789012345678901234567890123456789012345678901234567890
//          1         2         3         4         5         6
//    log.Error(log.Attrs("parse-error", err, "source-line", line, "column-index", pos.column), m)
//^^     ^     ^         ^^^           ^      ^           ^      ^        ^                  ^  ^^
//  println()
//  ^^     ^^

//pos.column = 1 -> 1
//    log.Error(log.Attrs("parse-error", err, "so…
//^
//
//pos.column = 2 -> 2
//    log.Error(log.Attrs("parse-error", err, "so…
// ^
//
//pos.column = 8 -> 8
//    log.Error(log.Attrs("parse-error", err, "so…
//       ^
//
//pos.column = 14 -> 14
//    log.Error(log.Attrs("parse-error", err, "so…
//             ^
//
//pos.column = 24 -> 24
//    log.Error(log.Attrs("parse-error", err, "so…
//                       ^
//
//pos.column = 25 -> 25
//    log.Error(log.Attrs("parse-error", err, "so…
//                        ^
//
//pos.column=26 source.marker.column=26 result.marker.column=25
//…  log.Error(log.Attrs("parse-error", err, "sou…
//                        ^
//
//pos.column = 38 -> 25
//…log.Attrs("parse-error", err, "source-line", l…
//                        ^
//
//pos.column = 45 -> 25
//…rs("parse-error", err, "source-line", line, "c…
//                        ^
//
//pos.column = 57 -> 25
//…ror", err, "source-line", line, "column-index"…
//                        ^
//
//pos.column = 64 -> 25
//…rr, "source-line", line, "column-index", pos.c…
//                        ^
//
//pos.column = 73 -> 25
//…ce-line", line, "column-index", pos.column), m)
//                        ^
//
//pos.column = 92 -> 44
//…ce-line", line, "column-index", pos.column), m)
//                                           ^
//
//pos.column = 95 -> 47
//…ce-line", line, "column-index", pos.column), m)
//                                              ^
//
//pos.column = 96 -> 48
//…ce-line", line, "column-index", pos.column), m)
//                                               ^

//pos.column = 3 -> 3
//  println()
//  ^
//
//pos.column = 4 -> 4
//  println()
//   ^
//
//pos.column = 10 -> 10
//  println()
//         ^
//
//pos.column = 11 -> 11
//  println()
//          ^
