package cli

import tea "charm.land/bubbletea/v2"

// logMsg is used to queue a message written to the REPL output stream.
//
// This is required for synchronizing I/O with the REPL whether alt-screen mode
// is enabled or not.
type logMsg struct {
	template string
	args     []any
}

// makeLogMsg is a helper for constructing logMsg with optional printf-style
// formatting.
//
// It replaces noisy struct and slice literals at call sites with concise
// function call syntax.
func makeLogMsg(template string, args ...any) logMsg {
	return logMsg{template, args}
}

type faultMsg struct{ err error }

func fault(err error) tea.Cmd {
	return func() tea.Msg { return faultMsg{err} }
}

type setEditModeMsg struct {
	mode editMode
	next []tea.Msg
}

func setEditMode(mode editMode, next ...tea.Msg) tea.Cmd {
	return func() tea.Msg { return setEditModeMsg{mode, next} }
}

// Executing REPL input requires 4 render cycles to synchronize the persistent
// models and output stream without visual artifacts.
type (
	collectMsg  struct{ input string }      // 1. save input; clear view
	captureMsg  struct{ input string }      // 2. create input snapshot; reset
	commitMsg   struct{ text, view string } // 3. draw input snapshot
	evaluateMsg struct{ input string }      // 4. evaluate input; draw output
)

func collect(input string) tea.Cmd     { return func() tea.Msg { return collectMsg{input} } }
func capture(input string) tea.Cmd     { return func() tea.Msg { return captureMsg{input} } }
func commit(text, view string) tea.Cmd { return func() tea.Msg { return commitMsg{text, view} } }
func evaluate(input string) tea.Cmd    { return func() tea.Msg { return evaluateMsg{input} } }

type (
	readyMsg struct{}
	resetMsg struct{}
	quitMsg  struct{}
)

func ready() tea.Msg { return readyMsg{} }
func reset() tea.Msg { return resetMsg{} }
func quit() tea.Msg  { return quitMsg{} }
