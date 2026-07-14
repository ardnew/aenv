//go:build goexperiment.jsonv2

package lang

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"unicode/utf8"

	"github.com/ardnew/aenv/log"
)

type Buffer []byte

func (b Buffer) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(b))
}

// AST represents the abstract syntax tree of a source.
type AST struct {
	B   Buffer `json:"src"`
	Pos Pos    `json:"pos"`
}

func (a *AST) Write(b []byte) (int, error) {
	log.Trace(log.Attrs("pos", a.Pos, "len", len(b)))

	// a.scan(b)
	a.B = make([]byte, len(b))
	copy(a.B, b)
	log.Debug(log.Attrs("pos", a.Pos))
	return len(b), nil
}

// parse reads source from r and appends its position state to the AST.
func (a *AST) parse(r io.Reader) (int64, error) {
	log.Trace(log.Attrs("pos", a.Pos))
	b, err := io.ReadAll(r)
	n := a.scan(b)
	log.Debug(log.Attrs("pos", a.Pos, "error", err))
	return n, err
}

func (a *AST) scan(b []byte) int64 {
	n := int64(len(b))
	a.B = append(a.B, b...)
	if n != 0 {
		if a.Pos.Line == 0 {
			a.Pos.Line = 1
		}
		if a.Pos.Column == 0 {    
			a.Pos.Column = 1
		}
		a.Pos.Offset += n
		if lastLine := bytes.LastIndexByte(b, '\n'); lastLine >= 0 {
			a.Pos.Line += int64(bytes.Count(b, []byte{'\n'}))
			a.Pos.Column = int64(utf8.RuneCount(b[lastLine+1:])) + 1
		} else {
			a.Pos.Column += int64(utf8.RuneCount(b))
		}
	}
	return n
}

func (a *AST) String() string {
	b, err := json.Marshal(a)
	if err != nil {
		log.Error(log.Attrs("error", err))
		return ""
	}
	return string(b)
}
