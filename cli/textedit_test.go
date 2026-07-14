package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTextEdit_DefaultModeIsArea(t *testing.T) {
	e := makeTextEdit()
	if got := e.mode; got != editArea {
		t.Fatalf("default mode = %v, want %v", got, editArea)
	}
}

func TestTextEdit_NextEditModeCycles(t *testing.T) {
	if got := editArea.next(); got != editLine {
		t.Fatalf("next mode from area = %v, want %v", got, editLine)
	}
	if got := editLine.next(); got != editArea {
		t.Fatalf("next mode from line = %v, want %v", got, editArea)
	}
}

func TestTextEdit_SetValueSyncsBothEditors(t *testing.T) {
	e := makeTextEdit()
	e = e.setValue("foo\nbar")

	if got := e.area.Value(); got != "foo\nbar" {
		t.Fatalf("area value = %q, want %q", got, "foo\\nbar")
	}
	if got := e.line.Value(); got != "foo bar" {
		t.Fatalf("line value = %q, want %q", got, "foo bar")
	}
}

func TestTextEdit_CurrentValueRespectsMode(t *testing.T) {
	e := makeTextEdit()
	e = e.setValue("a\nb")

	e.mode = editArea
	if got := e.value(); got != "a\nb" {
		t.Fatalf("area current value = %q, want %q", got, "a\\nb")
	}

	e.mode = editLine
	if got := e.value(); got != "a b" {
		t.Fatalf("line current value = %q, want %q", got, "a b")
	}
}

func TestTextEdit_UpdateDelegatesByMode(t *testing.T) {
	e := makeTextEdit()
	e.mode = editLine
	e, _ = e.setFocus(editLine)
	next, _ := e.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	updated, ok := next.(TextEdit)
	if !ok {
		t.Fatalf("Update() model = %T, want TextEdit", next)
	}
	if got := updated.value(); got != "x" {
		t.Fatalf("line value after update = %q, want %q", got, "x")
	}

	updated = updated.setValue("x")
	updated.mode = editArea
	updated, _ = updated.setFocus(editArea)
	next, _ = updated.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	updated, ok = next.(TextEdit)
	if !ok {
		t.Fatalf("Update() model = %T, want TextEdit", next)
	}
	if got := updated.value(); got != "xy" {
		t.Fatalf("area value after update = %q, want %q", got, "xy")
	}
}

func TestTextEdit_SetValuePreservesLineCursor(t *testing.T) {
	e := makeTextEdit()
	e = e.setValue("abc")
	e.mode = editLine
	e, _ = e.setFocus(editLine)
	e.line.SetCursor(1)

	e = e.setValue("abc")

	if got := e.line.Position(); got != 1 {
		t.Fatalf("line cursor position after setValue = %d, want %d", got, 1)
	}
}

func TestTextEdit_CursorState_LineToArea(t *testing.T) {
	e := makeTextEdit()
	e = e.setValue("abc")
	e.mode = editLine
	e, _ = e.setFocus(editLine)
	e.line.SetCursor(1)

	e = e.setMode(editArea)
	e, _ = e.setFocus(e.mode)

	if got := e.area.Line(); got != 0 {
		t.Fatalf("area line = %d, want %d", got, 0)
	}
	if got := e.area.Column(); got != 1 {
		t.Fatalf("area column = %d, want %d", got, 1)
	}
}

func TestTextEdit_CursorState_AreaToLine(t *testing.T) {
	e := makeTextEdit()
	e = e.setValue("ab\ncd\nef")
	e.mode = editArea
	e, _ = e.setFocus(editArea)
	e.area.MoveToBegin()
	e.area.CursorDown()
	e.area.SetCursorColumn(1)

	e = e.setMode(editLine)
	e, _ = e.setFocus(e.mode)

	if got := e.line.Position(); got != 4 {
		t.Fatalf("line cursor position = %d, want %d", got, 4)
	}
}

func TestTextEdit_CursorState_AreaLineAreaRoundTripPreservesFormat(t *testing.T) {
	e := makeTextEdit()
	e = e.setValue("ab\ncd\nef")
	e.mode = editArea
	e, _ = e.setFocus(editArea)
	e.area.MoveToBegin()
	e.area.CursorDown()
	e.area.SetCursorColumn(1)

	e = e.setMode(editLine)
	e, _ = e.setFocus(e.mode)
	e = e.setMode(editArea)
	e, _ = e.setFocus(e.mode)

	if got := e.area.Value(); got != "ab\ncd\nef" {
		t.Fatalf("area value after round trip = %q, want %q", got, "ab\\ncd\\nef")
	}
	if got := e.area.Line(); got != 1 {
		t.Fatalf("area line after round trip = %d, want %d", got, 1)
	}
	if got := e.area.Column(); got != 1 {
		t.Fatalf("area column after round trip = %d, want %d", got, 1)
	}
}
