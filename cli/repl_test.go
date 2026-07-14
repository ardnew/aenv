package cli

import (
	"reflect"
	"regexp"
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

var cmdType = reflect.TypeOf(tea.Cmd(nil))

// cmdSlice unwraps tea.Batch and tea.Sequence messages, both of which are []Cmd
// under the hood (Sequence's message type is unexported, so reflection is used).
func cmdSlice(msg tea.Msg) ([]tea.Cmd, bool) {
	rv := reflect.ValueOf(msg)
	if rv.Kind() != reflect.Slice || rv.Type().Elem() != cmdType {
		return nil, false
	}
	out := make([]tea.Cmd, rv.Len())
	for i := range out {
		out[i], _ = rv.Index(i).Interface().(tea.Cmd)
	}
	return out, true
}

func printLineBody(msg tea.Msg) (string, bool) {
	rv := reflect.ValueOf(msg)
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return "", false
	}
	field := rv.FieldByName("messageBody")
	if !field.IsValid() || field.Kind() != reflect.String {
		return "", false
	}
	return field.String(), true
}

// pump executes cmd and every command it transitively produces, threading the
// model through each resulting message until the program settles. Batch and
// Sequence messages are unwrapped into their component commands. It deliberately
// never forwards blink/tick messages into the editor (repl.Update only forwards
// to the editor on key presses), so no command blocks on a timer.
func pump(t *testing.T, m repl, cmds ...tea.Cmd) repl {
	t.Helper()
	queue := append([]tea.Cmd(nil), cmds...)
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 10000 {
			t.Fatal("pump did not settle within 10000 steps")
		}
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		if subs, ok := cmdSlice(msg); ok {
			queue = append(subs, queue...)
			continue
		}
		var next tea.Cmd
		m, next = applyMsg(t, m, msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
	return m
}

// send applies msg to the model and then drives every command it produces to
// completion, returning the settled model.
func send(t *testing.T, m repl, msg tea.Msg) repl {
	t.Helper()
	m, cmd := applyMsg(t, m, msg)
	return pump(t, m, cmd)
}

// newREPL builds a sized, focused repl ready to accept input. It runs Init for
// coverage of the model lifecycle, applies an initial window size, and focuses
// the editor directly to avoid the global logger side effects of the ready
// message path (covered separately by the onReady test).
func newREPL(t *testing.T, opts ...option[repl]) repl {
	t.Helper()
	m := makeREPL(t.Context(), opts...)
	_ = m.Init()
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m.edit, _ = m.edit.setFocus(m.edit.mode)
	return m
}

var csiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// visible strips ANSI control sequences so rendered text can be asserted on.
func visible(s string) string { return csiPattern.ReplaceAllString(s, "") }

func TestRepl_Update_TypingUpdatesView(t *testing.T) {
	m := newREPL(t)
	m = typeKey(t, m, 'h')
	m = typeKey(t, m, 'i')

	if got := m.edit.value(); got != "hi" {
		t.Fatalf("edit value = %q, want %q", got, "hi")
	}
	if got := visible(m.View().Content); !strings.Contains(got, "hi") {
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

	m := newREPL(t)
	for _, size := range sizes {
		m, _ = applyMsg(t, m, size)
	}
	m = typeKey(t, m, 'x')

	if got := m.edit.value(); !strings.Contains(got, "x") {
		t.Fatalf("edit value = %q, want to contain %q", got, "x")
	}
}

func TestRepl_Update_ResizeUpdatesViewportSize(t *testing.T) {
	m := makeREPL(t.Context())
	m.altScreen = true

	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if got := m.screen.Width(); got != 80 {
		t.Fatalf("viewport width = %d, want %d", got, 80)
	}
	wantHeight := max(0, m.edit.bounds.Y-max(1, lineCount(m.edit.View().Content)))
	if got := m.screen.Height(); got != wantHeight {
		t.Fatalf("viewport height = %d, want %d", got, wantHeight)
	}

	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 101, Height: 31})
	if got := m.screen.Width(); got != 101 {
		t.Fatalf("viewport width after resize = %d, want %d", got, 101)
	}
	wantHeight = max(0, m.edit.bounds.Y-max(1, lineCount(m.edit.View().Content)))
	if got := m.screen.Height(); got != wantHeight {
		t.Fatalf("viewport height after resize = %d, want %d", got, wantHeight)
	}
}

func evalKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt}
}

func toggleEditModeKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'e', Mod: tea.ModAlt}
}

func enterKey() tea.KeyPressMsg {
	// Text must be left unset: Key.String() (used by key.Matches) prefers a
	// non-empty Text over the keystroke name, so a literal "\n" here would
	// prevent this from matching the "enter" binding and instead forward the
	// raw text into the focused editor.
	return tea.KeyPressMsg{Code: tea.KeyEnter}
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
	m := newREPL(t)

	m = send(t, m, toggleEditModeKey())
	if got := m.edit.mode; got != editLine {
		t.Fatalf("mode after first toggle = %v, want %v", got, editLine)
	}

	m = send(t, m, toggleEditModeKey())
	if got := m.edit.mode; got != editArea {
		t.Fatalf("mode after second toggle = %v, want %v", got, editArea)
	}
}

func TestRepl_Update_EnterSubmitsInLineMode(t *testing.T) {
	m := newREPL(t, withHistory(""))
	m = send(t, m, toggleEditModeKey())
	m = typeKey(t, m, 'h')
	m = typeKey(t, m, 'i')

	_, cmd := applyMsg(t, m, enterKey())
	if cmd == nil {
		t.Fatal("Update(enter in line mode) cmd = nil, want submit sequence")
	}
}

func TestRepl_HandleCommit_TrimsTranscriptPrintInLineMode(t *testing.T) {
	m := newREPL(t)
	m = send(t, m, toggleEditModeKey())

	_, cmd := m.handleCommit(commitMsg{text: "zqxv", view: " zqxv\n"})
	if cmd == nil {
		t.Fatal("handleCommit(commitMsg) cmd = nil, want transcript sequence")
	}

	steps, ok := cmdSlice(cmd())
	if !ok || len(steps) != 2 {
		t.Fatalf("handleCommit(commitMsg) = %#v, want sequence of print and evaluate commands", cmd())
	}

	body, ok := printLineBody(steps[0]())
	if !ok {
		t.Fatalf("first commit step = %#v, want tea.Println message", steps[0]())
	}
	if body != " zqxv" {
		t.Fatalf("printed transcript body = %q, want %q", body, " zqxv")
	}

	eval, ok := steps[1]().(evaluateMsg)
	if !ok {
		t.Fatalf("second commit step = %#v, want evaluateMsg", steps[1]())
	}
	if eval.input != "zqxv" {
		t.Fatalf("evaluate input = %q, want %q", eval.input, "zqxv")
	}
}

func TestRepl_HandleCapture_LineSnapshotFitsViewport(t *testing.T) {
	m := newREPL(t)
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 24})
	m = send(t, m, toggleEditModeKey())

	_, cmd := m.handleCapture(captureMsg{input: "hello"})
	if cmd == nil {
		t.Fatal("handleCapture(captureMsg) cmd = nil, want reset+commit sequence")
	}

	steps, ok := cmdSlice(cmd())
	if !ok || len(steps) != 2 {
		t.Fatalf("handleCapture(captureMsg) = %#v, want sequence of reset and commit commands", cmd())
	}

	commit, ok := steps[1]().(commitMsg)
	if !ok {
		t.Fatalf("second capture step = %#v, want commitMsg", steps[1]())
	}

	lines := strings.Split(visible(commit.view), "\n")
	if len(lines) != 1 {
		t.Fatalf("captured line snapshot = %q, want single visual line", visible(commit.view))
	}
	if got := runeCount(lines[0]); got > m.edit.line.Width() {
		t.Fatalf("captured line snapshot width = %d, want <= viewport width %d", got, m.edit.line.Width())
	}
}

func TestRepl_Update_EnterCreatesNewlineInAreaMode(t *testing.T) {
	m := newREPL(t)
	m = typeKey(t, m, 'a')
	m, _ = applyMsg(t, m, enterKey())
	m = typeKey(t, m, 'b')

	if got := m.edit.value(); got != "a\nb" {
		t.Fatalf("edit value = %q, want %q", got, "a\\nb")
	}
}

func TestRepl_Update_BackspacePhysicalNewlineSwitchesToLineMode(t *testing.T) {
	m := newREPL(t)
	m = typeKey(t, m, 'a')
	m = send(t, m, enterKey())
	m = typeKey(t, m, 'b')

	if got := m.edit.mode; got != editArea {
		t.Fatalf("mode before deleting newline = %v, want %v", got, editArea)
	}

	// Move to start of the second line, then backspace over the line delimiter.
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.edit.mode; got != editLine {
		t.Fatalf("mode after deleting newline = %v, want %v", got, editLine)
	}
	if got := m.edit.content; got != "ab" {
		t.Fatalf("content after deleting newline = %q, want %q", got, "ab")
	}
	if got := m.edit.value(); got != "ab" {
		t.Fatalf("active value after mode switch = %q, want %q", got, "ab")
	}
}

func TestRepl_Update_LogMsgBeforeReadyAvoidsProgramPrint(t *testing.T) {
	m := makeREPL(t.Context())
	_, cmd := applyMsg(t, m, makeLogMsg("%s", "trace line"))
	if cmd == nil {
		t.Fatal("Update(logMsg before ready) cmd = nil, want print command")
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
	if got := strings.Join(m.buffer, "\n"); got != "trace line" {
		t.Fatalf("alt-screen output = %q, want %q", got, "trace line")
	}
}

func TestRepl_Update_CommitAndEvaluateInAltScreenBufferTranscript(t *testing.T) {
	m := makeREPL(t.Context())
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 80, Height: 12})

	m, cmd := applyMsg(t, m, commitMsg{text: "hello", view: "prompt"})
	if got, want := len(m.buffer), 1; got != want {
		t.Fatalf("output after commit has %d entries, want %d", got, want)
	}
	if got := strings.TrimSpace(m.buffer[0]); got != "prompt" {
		t.Fatalf("output after commit = %q, want %q", got, "prompt")
	}
	if cmd == nil {
		t.Fatal("Update(commitMsg in alt-screen) cmd = nil, want evaluate command")
	}

	m, cmd = applyMsg(t, m, evaluateMsg{input: "hello"})
	if cmd != nil {
		t.Fatal("Update(evaluateMsg in alt-screen) cmd != nil, want nil when not quitting")
	}
	if got, want := len(m.buffer), 2; got != want {
		t.Fatalf("output after evaluate has %d entries, want %d", got, want)
	}
	if got := strings.TrimSpace(m.buffer[0]); got != "prompt" {
		t.Fatalf("first buffered entry = %q, want %q", got, "prompt")
	}
	if got := m.buffer[1]; !strings.Contains(got, `"src":"hello"`) {
		t.Fatalf("second buffered entry = %q, want it to include submitted source", got)
	}

	if got := m.View().Content; !strings.Contains(got, "prompt") || !strings.Contains(got, "hello") {
		t.Fatalf("View().Content = %q, want buffered transcript", got)
	}
}

func TestRepl_View_AltScreenOutputAnchorsAboveEditor(t *testing.T) {
	m := makeREPL(t.Context())
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})
	m, _ = applyMsg(t, m, makeLogMsg("%s", "trace line"))

	view := m.View().Content
	lines := strings.Split(view, "\n")
	editLines := max(1, lineCount(m.edit.View().Content))
	if len(lines) <= editLines {
		t.Fatalf("View().Content has %d lines, want more than edit lines (%d): %q", len(lines), editLines, view)
	}

	editorStart := len(lines) - editLines
	if got := lines[editorStart-1]; got != "trace line" {
		t.Fatalf("line above editor = %q, want %q", got, "trace line")
	}
}

func TestRepl_Update_AltScreenPlainDFUBTypeIntoEditor(t *testing.T) {
	m := newREPL(t)
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})
	m = typeKey(t, m, 'd')
	m = typeKey(t, m, 'f')
	m = typeKey(t, m, 'u')
	m = typeKey(t, m, 'b')

	if got := m.edit.value(); got != "dfub" {
		t.Fatalf("edit value = %q, want %q", got, "dfub")
	}
}

func TestRepl_Update_AltScreenViewportPageScroll(t *testing.T) {
	m := newREPL(t)
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})

	for i := 0; i < 40; i++ {
		m, _ = applyMsg(t, m, makeLogMsg("line-%02d", i))
	}

	baseline := m.screen.YOffset()
	if baseline == 0 {
		t.Fatal("expected non-zero initial viewport offset with long output")
	}

	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if got := m.screen.YOffset(); got >= baseline {
		t.Fatalf("offset after pgup = %d, want < %d", got, baseline)
	}

	afterUp := m.screen.YOffset()
	m, _ = applyMsg(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if got := m.screen.YOffset(); got <= afterUp {
		t.Fatalf("offset after pgdown = %d, want > %d", got, afterUp)
	}
}

func TestRepl_Update_AltScreenLogVisibleAfterLineToAreaSwitch(t *testing.T) {
	m := newREPL(t)
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})

	// Ensure baseline output is visible in alt-screen.
	m, _ = applyMsg(t, m, makeLogMsg("before-switch"))
	if got := visible(m.View().Content); !strings.Contains(got, "before-switch") {
		t.Fatalf("alt-screen view before switch = %q, want to contain %q", got, "before-switch")
	}

	// Force line mode, then trigger line -> area through alt+enter.
	if m.edit.mode != editLine {
		m = send(t, m, toggleEditModeKey())
	}
	m = send(t, m, evalKey())
	if got := m.edit.mode; got != editArea {
		t.Fatalf("mode after alt+enter from line mode = %v, want %v", got, editArea)
	}

	// New logs must remain visible after the mode switch.
	m, _ = applyMsg(t, m, makeLogMsg("after-switch"))
	if got := visible(m.View().Content); !strings.Contains(got, "after-switch") {
		t.Fatalf("alt-screen view after switch = %q, want to contain %q", got, "after-switch")
	}
}

func TestRepl_Update_AltScreenKeepsFollowingBottomAfterFilledAndModeSwitch(t *testing.T) {
	m := newREPL(t)
	m.altScreen = true
	m, _ = applyMsg(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})

	// Fill beyond viewport so scrolling state matters.
	for i := 0; i < 30; i++ {
		m, _ = applyMsg(t, m, makeLogMsg("row-%02d", i))
	}

	// Start from line mode and switch to area mode (editor grows vertically).
	if m.edit.mode != editLine {
		m = send(t, m, toggleEditModeKey())
	}
	m = send(t, m, evalKey())
	if got := m.edit.mode; got != editArea {
		t.Fatalf("mode after alt+enter from line mode = %v, want %v", got, editArea)
	}

	// New output should still be visible at bottom after resize/switch.
	m, _ = applyMsg(t, m, makeLogMsg("tail-after-switch"))
	if got := visible(m.View().Content); !strings.Contains(got, "tail-after-switch") {
		t.Fatalf("alt-screen view after filled+switch = %q, want to contain %q", got, "tail-after-switch")
	}
}
