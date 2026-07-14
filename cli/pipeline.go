package cli

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ardnew/aenv/log"
)

func lineCaptureWidth(edit LineEdit, viewportWidth int) int {
	styles := edit.Styles()
	promptStyle := styles.Blurred.Prompt
	if edit.Focused() {
		promptStyle = styles.Focused.Prompt
	}
	promptWidth := lipgloss.Width(promptStyle.Render(edit.Prompt))

	cursorWidth := 0
	if edit.VirtualCursor() {
		cursorWidth = lipgloss.Width(" ")
	}

	return max(0, viewportWidth-promptWidth-cursorWidth)
}

// Executing REPL input requires 4 render cycles to synchronize the persistent
// editor models and output stream without visual artifacts, followed by a
// reset of the live editor. This file holds the handler for each stage, kept
// together (rather than split across concern-specific files) so the whole
// cycle can be read start-to-finish in one place. See msg.go for the
// corresponding collectMsg/captureMsg/commitMsg/evaluateMsg/resetMsg types.
//
//  1. handleCollect: normalize + record the submitted input to history, then
//     blur the editor so the next cycle can capture its unfocused appearance.
//  2. handleCapture: render a fresh, unfocused snapshot of the input using a
//     brand new editor model (not the live one -- see the comment inside for
//     why), then request a reset before committing that snapshot to the
//     output stream.
//  3. handleCommit: write the captured snapshot to the output stream/buffer,
//     then start evaluation.
//  4. handleEvaluate: feed the input to the AST, write the result to the
//     output stream/buffer, and quit if the user requested eval-and-quit.
//
// handleReset (triggered between capture and commit) clears and refocuses the
// live editor so it starts empty rather than carrying over the captured
// snapshot's state.

func (l repl) handleCollect(msg collectMsg) (repl, tea.Cmd) {
	// Normalize the input, save to history and forward to next command.
	text := strings.TrimRight(msg.input, "\r\n")
	log.Trace(msgAttr(msg,
		"mode", l.edit.mode,
		"len", len(text),
		"lines", lineCount(text),
	))
	// Write unstyled text to history so that it can be recalled directly
	// without formatting artifacts.
	l.hist.record(text)
	// Remove focus and capture the view in the next render cycle.
	var focus tea.Cmd
	l.edit, focus = l.edit.setValue("").setFocus(editNone)
	return l, tea.Sequence(focus, capture(text))
}

func (l repl) handleCapture(msg captureMsg) (repl, tea.Cmd) {
	log.Trace(msgAttr(msg, "mode", l.edit.mode))
	// Capture the appearance of the now-rendered unfocused edit model for
	// logging to the output stream (e.g., scrollback buffer).
	//
	// But first, we must reset the edit model so that the captured view draws
	// relative to a new, cleared edit model. Otherwise, the captured views
	// begin collecting vertical gaps when the edit model reaches the bottom of
	// the terminal window.
	var view tea.View
	switch l.edit.mode {
	case editLine:
		edit := makeLineEdit(msg.input).setStyle(l.edit.style.isDark)
		edit.SetWidth(lineCaptureWidth(edit, l.edit.line.Width()))
		view = edit.View()

	case editArea:
		edit := makeAreaEdit(msg.input)
		edit.SetWidth(l.edit.area.Width())
		view = edit.setStyle(l.edit.style.isDark).View()
	}
	return l, tea.Sequence(reset, commit(msg.input, view.Content))
}

func (l repl) handleCommit(msg commitMsg) (repl, tea.Cmd) {
	log.Trace(msgAttr(msg, "mode", l.edit.mode))
	if l.altScreen {
		l = l.appendOutput(msg.view)
		return l, evaluate(msg.text)
	}
	return l, tea.Sequence(
		tea.Println(strings.TrimRight(msg.view, "\r\n")),
		evaluate(msg.text),
	)
}

func (l repl) handleEvaluate(msg evaluateMsg) (repl, tea.Cmd) {
	log.Debug(msgAttr(msg, "mode", l.edit.mode))
	// evaluate is defined with a value receiver for immutability.
	r, output, err := l.evaluate(msg.input)
	if err != nil {
		// Return the original [repl] to avoid preserving an invalid or incomplete
		// AST in its model, which could otherwise reproduce related errors.
		return l, fault(err)
	}
	var batch []tea.Cmd
	if l.altScreen {
		r = r.appendOutput(output)
	} else {
		batch = append(batch, tea.Println(output))
	}
	if l.quitting {
		batch = append(batch, quit)
	}
	return r, tea.Sequence(batch...)
}

func (l repl) handleReset(msg resetMsg) (repl, tea.Cmd) {
	l.edit.mode = editLine
	log.Trace(msgAttr(msg, "mode", l.edit.mode))
	l.edit = l.edit.reset()
	var focus tea.Cmd
	l.edit, focus = l.edit.setFocus(l.edit.mode)
	return l.syncViewportSize(), focus
}

// evaluate feeds input to the REPL's AST and returns its resulting string
// representation. It is defined with a value receiver for immutability.
func (l repl) evaluate(input string) (repl, string, error) {
	attrs := log.Attrs(
		"len", len(input),
		"lines", lineCount(input),
	)
	log.Trace(attrs)

	_, err := strings.NewReader(input).WriteTo(&l.ast)
	if err != nil {
		log.Error(log.Attrs("error", err))
	}

	return l, l.ast.String(), nil
}
