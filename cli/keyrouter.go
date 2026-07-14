package cli

import (
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ardnew/aenv/log"
)

// keyMap holds all key bindings recognized by the REPL. See handleKeyPress
// for how each binding is routed to an action.
type keyMap struct {
	evalLine key.Binding
	evalArea key.Binding
	exec     key.Binding
	quit     key.Binding

	source  key.Binding
	format  key.Binding
	preview key.Binding
	screen  key.Binding
	toggle  key.Binding

	prev key.Binding
	next key.Binding
}

var defaultKeyMap = sync.OnceValue(
	func() keyMap {
		return keyMap{
			evalLine: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "eval"),
			),
			evalArea: key.NewBinding(
				key.WithKeys("alt+enter"),
				key.WithHelp("alt+enter", "eval"),
			),
			exec: key.NewBinding(
				key.WithKeys("ctrl+d"),
				key.WithHelp("ctrl+d", "eval and quit"),
			),
			quit: key.NewBinding(
				key.WithKeys("ctrl+c", "ctrl+q"),
				key.WithHelp("ctrl+c", "quit"),
			),
			source: key.NewBinding(
				key.WithKeys("ctrl+o"),
				key.WithHelp("ctrl+o", "source from file"),
			),
			format: key.NewBinding(
				key.WithKeys("ctrl+f"),
				key.WithHelp("ctrl+f", "format input"),
			),
			preview: key.NewBinding(
				key.WithKeys("ctrl+p"),
				key.WithHelp("ctrl+p", "preview"),
			),
			screen: key.NewBinding(
				key.WithKeys("alt+s"),
				key.WithHelp("alt+s", "toggle alt-screen"),
			),
			toggle: key.NewBinding(
				key.WithKeys("alt+e"),
				key.WithHelp("alt+e", "toggle input mode"),
			),
			prev: key.NewBinding(
				key.WithKeys("up"),
				key.WithHelp("up", "previous (history)"),
			),
			next: key.NewBinding(
				key.WithKeys("down"),
				key.WithHelp("down", "next (history)"),
			),
		}
	},
)

// handleKeyPress routes a key press to the action bound to it (see keyMap),
// forwarding the key to the active text editor when it isn't consumed by any
// bound action.
//
// NOTE: this is the original case body from Update's tea.KeyPressMsg case,
// which fell through to a shared tail (optionally forward to the editor, then
// syncViewportSize) rather than returning early. That tail is now reproduced
// explicitly at the end of this method.
func (l repl) handleKeyPress(msg tea.KeyPressMsg) (repl, tea.Cmd) {
	log.Trace(msgAttr(msg, "code", msg.Code, "text", msg.Text, "mod", msg.Mod))

	isLineMode := l.edit.mode != editArea
	forwardText := true
	var cmd tea.Cmd

	switch {
	case l.altScreen &&
		(key.Matches(msg, l.screen.KeyMap.PageDown) ||
			key.Matches(msg, l.screen.KeyMap.PageUp) ||
			key.Matches(msg, l.screen.KeyMap.HalfPageDown) ||
			key.Matches(msg, l.screen.KeyMap.HalfPageUp)):
		l.screen, cmd = l.screen.Update(msg)
		forwardText = false

	case key.Matches(msg, l.keys.toggle):
		log.Debug(msgAttr(msg, "action", "toggle", "edit-mode", l.edit.mode))
		return l, setEditMode(l.edit.mode.next())

	case key.Matches(msg, l.keys.evalLine):
		if isLineMode {
			log.Debug(msgAttr(msg, "action", "eval"))
			return l, collect(l.edit.value())
		}

	case key.Matches(msg, l.keys.evalArea):
		switch l.edit.mode {
		case editLine:
			// We have to forward the keypress in the setEditModeMsg so that it can
			// be processed by the editArea model after the mode switch.
			// Also, clear modifiers to avoid forwarding the exact same event.
			msg.Mod = 0
			return l, setEditMode(editArea, msg)
		case editArea:
			log.Debug(msgAttr(msg, "action", "eval"))
			return l, collect(l.edit.value())
		}

	case key.Matches(msg, l.keys.exec):
		log.Debug(msgAttr(msg, "action", "eval and quit"))
		l.quitting = true
		return l, collect(l.edit.value())

	case key.Matches(msg, l.keys.quit):
		log.Debug(msgAttr(msg, "action", "quit"))
		return l, tea.Quit

	case key.Matches(msg, l.keys.source):
		log.Debug(msgAttr(msg, "action", "source from file"))

	case key.Matches(msg, l.keys.format):
		log.Debug(msgAttr(msg, "action", "format input"))

	case key.Matches(msg, l.keys.preview):
		log.Debug(msgAttr(msg, "action", "preview"))

	case key.Matches(msg, l.keys.screen):
		log.Debug(msgAttr(msg, "action", "toggle", "alt-screen", !l.altScreen))
		l.altScreen = !l.altScreen
		l = l.syncViewportSize()

	case key.Matches(msg, l.keys.prev):
		if l.edit.atFirstLine() {
			if value, ok := l.hist.prev(l.edit.value()); ok {
				l.edit = l.edit.setValue(value).moveCursorEnd()
			}
			forwardText = false
		}

	case key.Matches(msg, l.keys.next):
		if l.edit.atLastLine() {
			if value, ok := l.hist.next(); ok {
				l.edit = l.edit.setValue(value).moveCursorEnd()
			}
			forwardText = false
		}
	}

	if forwardText {
		edit, editCmd := l.edit.Update(msg)
		if text, ok := edit.(TextEdit); ok {
			l.edit = text
		}
		cmd = tea.Batch(cmd, editCmd)
	}
	return l.syncViewportSize(), cmd
}

// handleSetEditMode applies a mode switch requested via setEditMode (e.g. by
// the toggle key, or by evalArea when switching from line to area mode), then
// replays any queued follow-up messages -- typically the key press that
// triggered the switch, so it can be reprocessed by the newly active editor.
func (l repl) handleSetEditMode(msg setEditModeMsg) (repl, tea.Cmd) {
	log.Trace(msgAttr(msg, "mode", msg.mode))

	value := l.edit.content
	l.edit = l.edit.setValue(value)
	l.edit = l.edit.setMode(msg.mode)

	var focus tea.Cmd
	l.edit, focus = l.edit.setFocus(msg.mode)

	cmds := []tea.Cmd{focus}
	for _, m := range msg.next {
		cmds = append(cmds, func() tea.Msg { return m })
	}

	return l.syncViewportSize(), tea.Batch(cmds...)
}
