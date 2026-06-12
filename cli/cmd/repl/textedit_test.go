package repl

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTextEdit_DefaultMode(t *testing.T) {
	edit := makeTextEdit(">", 40)

	if edit.Mode() != editArea {
		t.Fatalf("default mode = %v, want %v", edit.Mode(), editArea)
	}
}

func TestTextEdit_TogglePreservesContent(t *testing.T) {
	edit := makeTextEdit(">", 40)
	want := "foo\nbar"
	edit.SetValue(want)

	edit.ToggleMode()
	if got := edit.Value(); got != want {
		t.Fatalf("value after area->line toggle = %q, want %q", got, want)
	}

	edit.ToggleMode()
	if got := edit.Value(); got != want {
		t.Fatalf("value after line->area toggle = %q, want %q", got, want)
	}
}

func TestTextEdit_AreaEnterInsertsNewline(t *testing.T) {
	edit := makeTextEdit(">", 40)
	edit.SetValue("foo")
	edit.SetCursor(len("foo"))

	updated, _ := edit.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.Value(); got != "foo\n" {
		t.Fatalf("value after enter in area mode = %q, want %q", got, "foo\n")
	}
}
