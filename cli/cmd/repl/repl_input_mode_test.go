package repl

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

func newInputModeTestModel(t *testing.T) model {
	t.Helper()

	ast, err := lang.ParseString(context.Background(), `a:1`)
	if err != nil {
		t.Fatalf("parse test AST: %v", err)
	}

	history := NewHistory(filepath.Join(t.TempDir(), "history.utf8"))
	logger := log.Make(io.Discard)

	return newModel(context.Background(), ast, history, logger)
}

func TestModel_DefaultEditorModeIsArea(t *testing.T) {
	m := newInputModeTestModel(t)

	if got := m.edit.Mode(); got != editArea {
		t.Fatalf("default editor mode = %v, want %v", got, editArea)
	}
}

func TestModel_AltE_TogglesEditorMode(t *testing.T) {
	m := newInputModeTestModel(t)

	got, _ := m.handleKey(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'e'},
		Alt:   true,
	})
	if got.edit.Mode() != editLine {
		t.Fatalf("mode after first Alt+E = %v, want %v", got.edit.Mode(), editLine)
	}

	got, _ = got.handleKey(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'e'},
		Alt:   true,
	})
	if got.edit.Mode() != editArea {
		t.Fatalf("mode after second Alt+E = %v, want %v", got.edit.Mode(), editArea)
	}
}

func TestModel_AreaEnterVsAltEnter(t *testing.T) {
	m := newInputModeTestModel(t)
	m.edit.SetValue("a")

	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got.edit.Value() != "a\n" {
		t.Fatalf("enter in area mode should add newline, got %q", got.edit.Value())
	}
	if got.history.Len() != 0 {
		t.Fatalf("enter in area mode should not submit, history len=%d", got.history.Len())
	}

	got.edit.SetValue("a")
	got, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	if got.edit.Value() != "" {
		t.Fatalf("alt+enter in area mode should submit and clear input, got %q", got.edit.Value())
	}
	if got.history.Len() != 1 {
		t.Fatalf("alt+enter in area mode should submit, history len=%d", got.history.Len())
	}
}

func TestModel_LineEnterSubmits(t *testing.T) {
	m := newInputModeTestModel(t)
	m.edit.ToggleMode()
	m.edit.SetValue("a")

	got, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got.edit.Value() != "" {
		t.Fatalf("enter in line mode should submit and clear input, got %q", got.edit.Value())
	}
	if got.history.Len() != 1 {
		t.Fatalf("enter in line mode should submit, history len=%d", got.history.Len())
	}
}

func TestModel_AltETogglePreservesNewlines(t *testing.T) {
	m := newInputModeTestModel(t)
	want := "foo\nbar"
	m.edit.SetValue(want)

	got, _ := m.handleKey(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'e'},
		Alt:   true,
	})
	if got.edit.Value() != want {
		t.Fatalf("area->line toggle value = %q, want %q", got.edit.Value(), want)
	}

	got, _ = got.handleKey(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'e'},
		Alt:   true,
	})
	if got.edit.Value() != want {
		t.Fatalf("line->area toggle value = %q, want %q", got.edit.Value(), want)
	}
}
