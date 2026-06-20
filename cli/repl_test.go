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
	m := makeREPL(t.Context())
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = typeKey(t, m, 'h')
	m = typeKey(t, m, 'i')

	if got := m.View().Content; !strings.Contains(got, "hi") {
		t.Fatalf("View().Content = %q, want to contain %q", got, "hi")
	}
}

func TestRepl_Update_CtrlCQuits(t *testing.T) {
	m := makeREPL(t.Context())
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

	m := makeREPL(t.Context())
	for _, size := range sizes {
		m, _ = applyMsg(t, m, size)
	}
	m = typeKey(t, m, 'x')

	if got := m.View().Content; !strings.Contains(got, "x") {
		t.Fatalf("View().Content = %q, want to contain %q", got, "x")
	}
}

func evalKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt}
}

func toggleEditModeKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'e', Text: "e", Mod: tea.ModAlt}
}

func enterKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter, Text: "\n"}
}

func TestRepl_Update_EvalRecordsHistoryAndClears(t *testing.T) {
	m := makeREPL(t.Context(), withHistory(""))
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = typeKey(t, m, 'h')
	m = typeKey(t, m, 'i')

	_, cmd := applyMsg(t, m, evalKey())

	if cmd == nil {
		t.Fatal("Update(eval) cmd = nil, want transcript command")
	}
}

func TestRepl_DefaultInputModeIsArea(t *testing.T) {
	m := makeREPL(t.Context())
	if got := m.edit.mode; got != editArea {
		t.Fatalf("default edit mode = %v, want %v", got, editArea)
	}
}

func TestRepl_Update_AltETogglesInputMode(t *testing.T) {
	m := makeREPL(t.Context())

	m, _ = applyMsg(t, m, toggleEditModeKey())
	if got := m.edit.mode; got != editLine {
		t.Fatalf("mode after first toggle = %v, want %v", got, editLine)
	}

	m, _ = applyMsg(t, m, toggleEditModeKey())
	if got := m.edit.mode; got != editArea {
		t.Fatalf("mode after second toggle = %v, want %v", got, editArea)
	}
}

func TestRepl_Update_EnterSubmitsInLineMode(t *testing.T) {
	m := makeREPL(t.Context(), withHistory(""))
	m, _ = applyMsg(t, m, toggleEditModeKey())
	m = typeKey(t, m, 'h')
	m = typeKey(t, m, 'i')

	_, cmd := applyMsg(t, m, enterKey())
	if cmd == nil {
		t.Fatal("Update(enter in line mode) cmd = nil, want submit sequence")
	}
}

func TestRepl_Update_EnterCreatesNewlineInAreaMode(t *testing.T) {
	m := makeREPL(t.Context())
	m = typeKey(t, m, 'a')
	m, _ = applyMsg(t, m, enterKey())
	m = typeKey(t, m, 'b')

	if got := m.edit.currentValue(); got != "a\nb" {
		t.Fatalf("edit value = %q, want %q", got, "a\\nb")
	}
}

func TestRepl_Update_LogMsgBeforeReadyAvoidsProgramPrint(t *testing.T) {
	m := makeREPL(t.Context())
	_, cmd := applyMsg(t, m, makeLogMsg("%s", "trace line"))
	if cmd != nil {
		t.Fatal("Update(logMsg before ready) cmd != nil, want nil")
	}
}

func TestRepl_Update_LogMsgAfterReadyUsesProgramPrint(t *testing.T) {
	m := makeREPL(t.Context())
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := applyMsg(t, m, makeLogMsg("%s", "trace line"))
	if cmd == nil {
		t.Fatal("Update(logMsg after ready) cmd = nil, want print command")
	}
}

func TestRepl_Update_LogMsgInAltScreenUsesProgramPrint(t *testing.T) {
	m := makeREPL(t.Context())
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	m, cmd := applyMsg(t, m, makeLogMsg("%s", "trace line"))
	if cmd != nil {
		t.Fatal("Update(logMsg in alt-screen) cmd != nil, want nil")
	}
	if got := strings.Join(m.output, "\n"); got != "trace line" {
		t.Fatalf("alt-screen output = %q, want %q", got, "trace line")
	}
}

func TestRepl_Update_CommitAndEvaluateInAltScreenBufferTranscript(t *testing.T) {
	m := makeREPL(t.Context())
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 12})

	m, cmd := applyMsg(t, m, commitMsg{text: "hello", view: "prompt"})
	if got := strings.Join(m.output, "\n"); got != "prompt" {
		t.Fatalf("output after commit = %q, want %q", got, "prompt")
	}
	if cmd == nil {
		t.Fatal("Update(commitMsg in alt-screen) cmd = nil, want evaluate command")
	}

	m, cmd = applyMsg(t, m, evaluateMsg{input: "hello"})
	if cmd != nil {
		t.Fatal("Update(evaluateMsg in alt-screen) cmd != nil, want nil when not quitting")
	}
	if got := strings.Join(m.output, "\n"); got != "prompt\nhello" {
		t.Fatalf("output after evaluate = %q, want %q", got, "prompt\\nhello")
	}

	if got := m.View().Content; !strings.Contains(got, "prompt") || !strings.Contains(got, "hello") {
		t.Fatalf("View().Content = %q, want buffered transcript", got)
	}
}
