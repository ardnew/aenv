package cli

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func applyMsg(t *testing.T, m repl, msg tea.Msg) (repl, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	updated, ok := next.(repl)
	if !ok {
		t.Fatalf("Update() model = %T, want repl", next)
	}
	return updated, cmd
}

func typeKey(t *testing.T, m repl, r rune) repl {
	t.Helper()
	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	return m
}

func TestRepl_Update_TypingUpdatesView(t *testing.T) {
	m := newREPL()
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = typeKey(t, m, 'h')
	m = typeKey(t, m, 'i')

	if got := m.View().Content; !strings.Contains(got, "hi") {
		t.Fatalf("View().Content = %q, want to contain %q", got, "hi")
	}
}

func TestRepl_Update_CtrlCQuits(t *testing.T) {
	m := newREPL()
	_, cmd := applyMsg(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Update(ctrl+c) cmd = nil, want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Update(ctrl+c) msg = %T, want tea.QuitMsg", cmd())
	}
}

func TestRepl_Update_HandlesSizeExtremes(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 0, Height: 1},
		{Width: 200, Height: 200},
	}

	m := newREPL()
	for _, size := range sizes {
		m, _ = applyMsg(t, m, size)
	}
	m = typeKey(t, m, 'x')

	if got := m.View().Content; !strings.Contains(got, "x") {
		t.Fatalf("View().Content = %q, want to contain %q", got, "x")
	}
}
