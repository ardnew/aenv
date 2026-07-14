package cli

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

var errTestFault = errors.New("test fault")

func ctrlKey(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }
func altKey(r rune) tea.KeyPressMsg  { return tea.KeyPressMsg{Code: r, Mod: tea.ModAlt} }

// --- TextEdit ---

func TestTextEdit_LifecycleAndAccessors(t *testing.T) {
	e := makeTextEdit()
	if e.mode != editArea {
		t.Fatalf("default mode = %v, want %v", e.mode, editArea)
	}
	_ = e.Init()
	e = e.setSize(tea.Position{X: 40, Y: 6})
	e = e.setStyle(true)
	e = e.setStyle(false)

	// area mode, multi-line
	e = e.setValue("a\nb\nc")
	if got := e.value(); got != "a\nb\nc" {
		t.Fatalf("area value = %q, want %q", got, "a\nb\nc")
	}
	_ = e.size()
	_ = e.View()
	e = e.moveCursorEnd()
	if !e.atLastLine() {
		t.Fatal("atLastLine() = false after moveCursorEnd, want true")
	}

	// line mode, single line: both first and last
	e = e.setMode(editLine)
	e = e.setValue("hello")
	if got := e.value(); got != "hello" {
		t.Fatalf("line value = %q, want %q", got, "hello")
	}
	if !e.atFirstLine() || !e.atLastLine() {
		t.Fatal("line mode single line should be both first and last")
	}
	_ = e.size()
	_ = e.View()

	// none mode falls back to stored content
	e = e.setMode(editNone)
	_ = e.value()
	_ = e.size()
	if !e.atFirstLine() || !e.atLastLine() {
		t.Fatal("non-area mode should report first and last line")
	}

	// reset clears the active binding
	e = e.setMode(editArea).setValue("x").reset()
	if got := e.value(); got != "" {
		t.Fatalf("value after reset = %q, want empty", got)
	}
}

func TestTextEdit_CursorCallback(t *testing.T) {
	e := makeTextEdit().setSize(tea.Position{X: 20, Y: 4})
	called := false
	_ = e.cursor(func(focused bool, c *tea.Cursor) { called = true })
	if !called {
		t.Fatal("cursor callback was not invoked")
	}

	// nil callback is a no-op.
	_ = e.cursor(nil)

	// line mode dispatches to the line editor.
	calledLine := false
	e = e.setMode(editLine)
	_ = e.cursor(func(focused bool, c *tea.Cursor) { calledLine = true })
	if !calledLine {
		t.Fatal("cursor callback was not invoked in line mode")
	}

	// none mode matches no case and does not invoke the callback.
	calledNone := false
	e = e.setMode(editNone)
	_ = e.cursor(func(focused bool, c *tea.Cursor) { calledNone = true })
	if calledNone {
		t.Fatal("cursor callback was invoked in none mode, want no-op")
	}
}

func TestTextEdit_MoveCursorEnd(t *testing.T) {
	e := makeTextEdit().setSize(tea.Position{X: 20, Y: 4})
	e = e.setMode(editLine).setValue("hello")
	e = e.moveCursorEnd()
	if !e.atLastLine() {
		t.Fatal("line mode moveCursorEnd did not report at last line")
	}
}

func TestEditMode_Cycle(t *testing.T) {
	if got := editArea.next(); got != editLine {
		t.Fatalf("editArea.next() = %v, want %v", got, editLine)
	}
	if got := editLine.next(); got != editArea {
		t.Fatalf("editLine.next() = %v, want %v", got, editArea)
	}
	if got := editNone.next(); got != editArea {
		t.Fatalf("editNone.next() = %v, want %v (default case)", got, editArea)
	}
}

// --- repl options & accessors ---

func TestRepl_OptionsAndAccessors(t *testing.T) {
	base := makeREPL(t.Context())
	m := makeREPL(t.Context(),
		withHistory(""),
		withKeyMap(defaultKeyMap()),
		withAST(base.ast),
	)
	if !m.IsTerminalWriter() {
		t.Fatal("IsTerminalWriter() = false, want true")
	}
}

func TestRepl_OnReady(t *testing.T) {
	restoreDefaultLogger(t)
	m := newREPL(t)
	m, err := m.onReady()
	if err != nil {
		t.Fatalf("onReady() error = %v", err)
	}
	_ = m
}

func TestRepl_Write_EnqueuesOntoLogQueue(t *testing.T) {
	m := newREPL(t)
	want := []byte("hello")
	n, err := m.Write(want)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(want) {
		t.Fatalf("Write() n = %d, want %d", n, len(want))
	}
	select {
	case got := <-m.logQ:
		if string(got) != string(want) {
			t.Fatalf("logq received %q, want %q", got, want)
		}
	default:
		t.Fatal("Write() did not enqueue bytes onto logq")
	}
}

func TestRepl_Update_ReadyMsgWiresLogSinkOnce(t *testing.T) {
	restoreDefaultLogger(t)
	m := newREPL(t)
	m, _ = applyMsg(t, m, readyMsg{})
	if !m.IsTerminalWriter() {
		t.Fatal("expected repl to report as terminal writer")
	}
	// A log line written after the sink is wired should be handed off to the
	// drain goroutine via logq rather than blocking the caller.
	if _, err := m.Write([]byte("after ready\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	// Re-delivering readyMsg (e.g. on reconnect) must not start a second
	// drain goroutine or otherwise fail; logOnce guards this.
	m, _ = applyMsg(t, m, readyMsg{})
	_ = m
}

// --- repl Update key routing ---

func TestRepl_Update_NoOpControlKeys(t *testing.T) {
	m := newREPL(t)
	for _, key := range []tea.KeyPressMsg{ctrlKey('o'), ctrlKey('f'), ctrlKey('p')} {
		m, _ = applyMsg(t, m, key)
	}
	// The editor should remain usable after no-op control keys.
	m = typeKey(t, m, 'z')
	if got := m.edit.value(); got == "" {
		t.Fatal("editor stopped accepting input after control keys")
	}
}

func TestRepl_Update_ScreenToggle(t *testing.T) {
	m := newREPL(t)
	if m.altScreen {
		t.Fatal("altScreen should start false")
	}
	m = send(t, m, altKey('s'))
	if !m.altScreen {
		t.Fatal("altScreen = false after toggle, want true")
	}
	m = send(t, m, altKey('s'))
	if m.altScreen {
		t.Fatal("altScreen = true after second toggle, want false")
	}
}

func TestRepl_Update_QuitKey(t *testing.T) {
	m := newREPL(t)
	_, cmd := applyMsg(t, m, ctrlKey('c'))
	if cmd == nil {
		t.Fatal("ctrl+c cmd = nil, want quit")
	}
}

func TestRepl_Update_ExecKeySetsQuitting(t *testing.T) {
	m := newREPL(t, withHistory(""))
	m = typeKey(t, m, 'q')
	m, cmd := applyMsg(t, m, ctrlKey('d'))
	if !m.quitting {
		t.Fatal("quitting = false after exec key, want true")
	}
	if cmd == nil {
		t.Fatal("exec cmd = nil, want submit sequence")
	}
}

func TestRepl_Update_EvalAreaKey(t *testing.T) {
	m := newREPL(t, withHistory(""))
	m = typeKey(t, m, 'x')
	_, cmd := applyMsg(t, m, evalKey())
	if cmd == nil {
		t.Fatal("alt+enter in area mode cmd = nil, want submit sequence")
	}
}

func TestRepl_Update_MouseWheelInAltScreen(t *testing.T) {
	m := newREPL(t)
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 6})
	for i := 0; i < 30; i++ {
		m, _ = applyMsg(t, m, makeLogMsg("row-%02d", i))
	}
	// Should not panic and should remain interactive.
	m, _ = applyMsg(t, m, tea.MouseWheelMsg{})
	_ = m.View()
}

func TestRepl_Update_FaultQuits(t *testing.T) {
	m := newREPL(t)
	_, cmd := applyMsg(t, m, faultMsg{err: errTestFault})
	if cmd == nil {
		t.Fatal("fault cmd = nil, want quit")
	}
}

func TestRepl_Update_LogMsgPlainPrintsLine(t *testing.T) {
	m := newREPL(t)
	if m.altScreen {
		t.Fatal("expected inline mode")
	}
	_, cmd := applyMsg(t, m, makeLogMsg("hello %s", "world"))
	if cmd == nil {
		t.Fatal("inline logMsg cmd = nil, want println")
	}
}

func TestRepl_Update_LogMsgAltScreenAppends(t *testing.T) {
	m := newREPL(t)
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 6})
	m, _ = applyMsg(t, m, makeLogMsg("appended line"))
	if got := visible(m.screen.View()); got == "" {
		t.Fatal("alt-screen viewport empty after append")
	}
}

// --- full evaluation cycle and history ---

func TestRepl_FullEvalCycleRecordsHistory(t *testing.T) {
	m := newREPL(t, withHistory(""))
	m = send(t, m, toggleEditModeKey()) // -> line mode
	for _, r := range "foo" {
		m = typeKey(t, m, r)
	}
	m = send(t, m, enterKey()) // collect -> capture -> commit -> evaluate -> reset

	if got := m.edit.value(); got != "" {
		t.Fatalf("editor not reset after eval, value = %q", got)
	}

	// History recall via prev/next.
	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if got := m.edit.value(); got != "foo" {
		t.Fatalf("history prev value = %q, want %q", got, "foo")
	}
	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	// next from the newest entry returns to an empty draft line.
	if got := m.edit.value(); got != "" {
		t.Fatalf("history next value = %q, want empty draft", got)
	}
}

func TestRepl_HistoryKeysForwardWhenNotAtEdge(t *testing.T) {
	m := newREPL(t)
	m = typeKey(t, m, 'a')
	m, _ = applyMsg(t, m, enterKey()) // newline in area mode -> two lines
	m = typeKey(t, m, 'b')

	// Cursor on last line: up should move within the editor (forwarded), not recall.
	before := m.edit.value()
	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m.edit.value(); got != before {
		t.Fatalf("multiline navigation mutated value: got %q want %q", got, before)
	}
}

// --- msg.go constructors ---

func TestMsg_Constructors(t *testing.T) {
	if got := fault(errTestFault)(); got != (faultMsg{errTestFault}) {
		t.Fatalf("fault(err)() = %#v, want %#v", got, faultMsg{errTestFault})
	}
	if got, ok := setEditMode(editLine)().(setEditModeMsg); !ok || got.mode != editLine || len(got.next) != 0 {
		t.Fatalf("setEditMode(mode)() = %#v, want mode=%v next=[]", got, editLine)
	}
	if got := collect("in")(); got != (collectMsg{"in"}) {
		t.Fatalf("collect(input)() = %#v, want %#v", got, collectMsg{"in"})
	}
	if got := capture("in")(); got != (captureMsg{"in"}) {
		t.Fatalf("capture(input)() = %#v, want %#v", got, captureMsg{"in"})
	}
	if got := commit("t", "v")(); got != (commitMsg{"t", "v"}) {
		t.Fatalf("commit(text, view)() = %#v, want %#v", got, commitMsg{"t", "v"})
	}
	if got := evaluate("in")(); got != (evaluateMsg{"in"}) {
		t.Fatalf("evaluate(input)() = %#v, want %#v", got, evaluateMsg{"in"})
	}
	if got := ready(); got != (readyMsg{}) {
		t.Fatalf("ready() = %#v, want %#v", got, readyMsg{})
	}
	if got := reset(); got != (resetMsg{}) {
		t.Fatalf("reset() = %#v, want %#v", got, resetMsg{})
	}
	if got := quit(); got != (quitMsg{}) {
		t.Fatalf("quit() = %#v, want %#v", got, quitMsg{})
	}
}

func TestRepl_Update_QuitCmdRunsAfterEvalAndQuit(t *testing.T) {
	m := newREPL(t, withHistory(""))
	m = typeKey(t, m, 'q')
	m, cmd := applyMsg(t, m, ctrlKey('d'))
	if !m.quitting {
		t.Fatal("quitting = false after exec key, want true")
	}
	m = pump(t, m, cmd)
	if !m.quitting {
		t.Fatal("quitting = false after pumping eval-and-quit sequence, want true")
	}
}

// --- option.go ---

func TestOption_WithOptionsComposesOptions(t *testing.T) {
	type target struct{ a, b int }
	setA := func(t *target) { t.a = 1 }
	setB := func(t *target) { t.b = 2 }
	got := wrap(target{}, withOptions(setA, setB))
	if got.a != 1 || got.b != 2 {
		t.Fatalf("wrap with withOptions() = %+v, want {a:1 b:2}", got)
	}
}

// --- editmode_string.go ---

func TestEditMode_String(t *testing.T) {
	cases := map[editMode]string{
		editNone: "no-editor",
		editLine: "line-editor",
		editArea: "multiline-editor",
	}
	for mode, want := range cases {
		if got := mode.String(); got != want {
			t.Fatalf("editMode(%d).String() = %q, want %q", mode, got, want)
		}
	}
	if got := editMode(-1).String(); got != "editMode(-1)" {
		t.Fatalf("editMode(-1).String() = %q, want %q", got, "editMode(-1)")
	}
	if got := editMode(99).String(); got != "editMode(99)" {
		t.Fatalf("editMode(99).String() = %q, want %q", got, "editMode(99)")
	}
}

// --- small pure helpers ---

func TestClamp(t *testing.T) {
	if got := clamp(5, 0, 10); got != 5 {
		t.Fatalf("clamp(5, 0, 10) = %d, want 5", got)
	}
	if got := clamp(-1, 0, 10); got != 0 {
		t.Fatalf("clamp(-1, 0, 10) = %d, want 0", got)
	}
	if got := clamp(11, 0, 10); got != 10 {
		t.Fatalf("clamp(11, 0, 10) = %d, want 10", got)
	}
}

func TestLineCount(t *testing.T) {
	if got := lineCount(""); got != 0 {
		t.Fatalf("lineCount(\"\") = %d, want 0", got)
	}
	if got := lineCount("a"); got != 1 {
		t.Fatalf("lineCount(%q) = %d, want 1", "a", got)
	}
	if got := lineCount("a\nb"); got != 2 {
		t.Fatalf("lineCount(%q) = %d, want 2", "a\nb", got)
	}
}

func TestTextEdit_SetValueAreaModeMultiline(t *testing.T) {
	e := makeTextEdit().setSize(tea.Position{X: 20, Y: 8})
	e = e.setMode(editArea).setValue("a\nb\nc")
	if got := e.value(); got != "a\nb\nc" {
		t.Fatalf("area value = %q, want %q", got, "a\nb\nc")
	}
	// Re-set to reposition the cursor onto an existing line.
	e = e.setValue("x\ny\nz")
	if got := e.value(); got != "x\ny\nz" {
		t.Fatalf("area value after re-set = %q, want %q", got, "x\ny\nz")
	}
}
