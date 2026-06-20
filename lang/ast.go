package lang

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ardnew/aenv/log"
)

// AST represents the abstract syntax tree of a source.
type AST struct {
	pos Pos
}

// ReadFrom parses from the provided reader and appends the AST to the receiver.
func (a *AST) ReadFrom(r io.Reader) (int64, error) {
	log.Trace(nil, "parse")
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanBytes)
	var count int64

	for s.Scan() {
		count += int64(len(s.Bytes()))
	}

	err := s.Err()
	attrs := log.Attrs("bytes", count)
	if err != nil {
		log.Debug(append(attrs, log.Attrs("error", err)...), "parse")
		return count, err
	}
	log.Debug(attrs, "parse")
	return count, nil
}

// Pos returns the position of the most recently parsed token in the AST.
func (a *AST) Pos() Pos { return a.pos }

// Pos refers to an absolute position in a byte stream in terms of byte offset
// and line:column, relative to the start of the stream.
//
// The offset is 0-based, while the line and column are 1-based.
// The first byte in a stream will be at offset=0 and line=column=1.
// This is distinguished from the zero value, which is an invalid position.
type Pos struct {
	offset, line, column int
}

// IsZero returns whether the position is the invalid zero value.
//
// The first byte in a stream will be at offset=0 and line=column=1.
func (p Pos) IsZero() bool {
	return p.offset == 0 && p.line == 0 && p.column == 0
}

// String returns a human-readable position string, e.g. "3:15+42",
// meaning line 3, column 15. The +42 byte offset refers to the same position.
func (p Pos) String() string {
	return fmt.Sprintf("%d:%d+%d", p.line, p.column, p.offset)
}
