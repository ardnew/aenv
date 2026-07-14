package lang

import "fmt"

// Pos refers to an absolute position in a byte stream in terms of byte offset
// and line:column, relative to the start of the stream.
//
// The offset is 0-based, while the line and column are 1-based.
// The first byte in a stream will be at offset=0 and line=column=1.
// This is distinguished from the zero value, which is an invalid position.
type Pos struct {
	Offset, Line, Column int64
}

// IsZero returns whether the position is the invalid zero value.
//
// The first byte in a stream will be at offset=0 and line=column=1.
func (p Pos) IsZero() bool {
	return p.Offset == 0 && p.Line == 0 && p.Column == 0
}

// Add returns a new Pos that is the field-wise sum of the receiver p and the
// given Pos q; i.e., p + q.
func (p Pos) Add(q Pos) Pos {
	return Pos{
		Offset: p.Offset + q.Offset,
		Line:   p.Line + q.Line,
		Column: p.Column + q.Column,
	}
}

// Sub returns a new Pos that is the field-wise difference of the receiver p
// and the provided Pos q; i.e., p - q.
func (p Pos) Sub(q Pos) Pos {
	return Pos{
		Offset: p.Offset - q.Offset,
		Line:   p.Line - q.Line,
		Column: p.Column - q.Column,
	}
}

// String returns a human-readable position string, e.g. "3:15+42",
// meaning line 3, column 15. The +42 byte offset refers to the same position.
func (p Pos) String() string {
	return fmt.Sprintf("%d:%d+%d", p.Line, p.Column, p.Offset)
}
