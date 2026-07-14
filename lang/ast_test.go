package lang

import (
	"strings"
	"testing"
)

func TestAST_ParseAdvancesPositionIncrementally(t *testing.T) {
	var a AST

	chunk := ""
	n, err := a.parse(strings.NewReader(chunk))
	if err != nil {
		t.Fatalf("parse first chunk: %v", err)
	}
	if want := int64(len(chunk)); n != want {
		t.Fatalf("first parse bytes = %d, want %d", n, want)
	}
	if got, want := a.Pos, (Pos{Offset: int64(len(chunk)), Line: 1, Column: 1}); got != want {
		t.Fatalf("first parse pos = %+v, want %+v", got, want)
	}

	chunk = "\n" + strings.Repeat("\u0301", 63) + "β"
	n, err = a.parse(strings.NewReader(chunk))
	if err != nil {
		t.Fatalf("parse first chunk: %v", err)
	}
	if want := int64(len(chunk)); n != want {
		t.Fatalf("first parse bytes = %d, want %d", n, want)
	}
	if got, want := a.Pos, (Pos{Offset: int64(len(chunk)), Line: 2, Column: 65}); got != want {
		t.Fatalf("first parse pos = %+v, want %+v", got, want)
	}

	chunk = "c"
	n, err = a.parse(strings.NewReader(chunk))
	if err != nil {
		t.Fatalf("parse second chunk: %v", err)
	}
	if want := int64(len(chunk)); n != want {
		t.Fatalf("second parse bytes = %d, want %d", n, want)
	}
	if got, want := a.Pos, (Pos{Offset: 130, Line: 2, Column: 66}); got != want {
		t.Fatalf("second parse pos = %+v, want %+v", got, want)
	}
}
