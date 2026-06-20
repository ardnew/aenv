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
	if got := nextEditMode(editArea); got != editLine {
		t.Fatalf("next mode from area = %v, want %v", got, editLine)
	}
	if got := nextEditMode(editLine); got != editArea {
		t.Fatalf("next mode from line = %v, want %v", got, editArea)
	}
}

func TestTextEdit_SetValueSyncsBothEditors(t *testing.T) {
	e := makeTextEdit().setValue("foo\nbar")

	if got := e.area.Value(); got != "foo\nbar" {
		t.Fatalf("area value = %q, want %q", got, "foo\\nbar")
	}
	if got := e.line.Value(); got != "foo bar" {
		t.Fatalf("line value = %q, want %q", got, "foo bar")
	}
}

func TestTextEdit_CurrentValueRespectsMode(t *testing.T) {
	e := makeTextEdit().setValue("a\nb")

	e.mode = editArea
	if got := e.currentValue(); got != "a\nb" {
		t.Fatalf("area current value = %q, want %q", got, "a\\nb")
	}

	e.mode = editLine
	if got := e.currentValue(); got != "a b" {
		t.Fatalf("line current value = %q, want %q", got, "a b")
	}
}

func TestTextEdit_UpdateDelegatesByMode(t *testing.T) {
	e := makeTextEdit()
	e.mode = editLine
	e = e.setFocus(editLine)
	next, _ := e.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	updated, ok := next.(TextEdit)
	if !ok {
		t.Fatalf("Update() model = %T, want TextEdit", next)
	}
	if got := updated.currentValue(); got != "x" {
		t.Fatalf("line value after update = %q, want %q", got, "x")
	}

	updated = updated.setValue("x")
	updated.mode = editArea
	updated = updated.setFocus(editArea)
	next, _ = updated.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	updated, ok = next.(TextEdit)
	if !ok {
		t.Fatalf("Update() model = %T, want TextEdit", next)
	}
	if got := updated.currentValue(); got != "xy" {
		t.Fatalf("area value after update = %q, want %q", got, "xy")
	}
}
