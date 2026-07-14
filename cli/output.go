package cli

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// appendOutput appends text (split into lines) to the REPL's output buffer
// and refreshes the viewport content.
//
// The buffer's joined string representation is cached in bufferText and
// updated incrementally here (append the delta instead of rejoining the
// entire buffer), since this runs once per log line/eval output in alt-screen
// mode. The full string is only rebuilt when maxOutputLines forces the front
// of the buffer to be trimmed, which is bounded and infrequent.
func (l repl) appendOutput(text string) repl {
	trimmed := strings.TrimRight(text, "\r\n")
	var added []string
	if trimmed == "" {
		added = []string{""}
	} else {
		added = strings.Split(trimmed, "\n")
	}
	l.buffer = append(l.buffer, added...)

	if drop := len(l.buffer) - maxOutputLines; drop > 0 {
		l.buffer = slices.Clone(l.buffer[drop:])
		l.bufferText = strings.Join(l.buffer, "\n")
	} else if l.bufferText == "" {
		l.bufferText = strings.Join(added, "\n")
	} else {
		l.bufferText += "\n" + strings.Join(added, "\n")
	}

	atBottom := l.screen.AtBottom()
	l.screen.SetContent(l.bufferText)
	if atBottom {
		l.screen.GotoBottom()
	}
	return l
}

// syncViewportSize resizes the output viewport to fill the space not
// occupied by the active editor.
func (l repl) syncViewportSize() repl {
	if l.edit.bounds.X <= 0 || l.edit.bounds.Y <= 0 {
		return l
	}
	atBottom := l.screen.AtBottom()
	editLines := max(1, lineCount(l.edit.View().Content))
	height := max(0, l.edit.bounds.Y-editLines)
	l.screen.SetWidth(l.edit.bounds.X)
	l.screen.SetHeight(height)
	if atBottom {
		l.screen.GotoBottom()
	}
	return l
}

// outputRegionView renders the currently visible slice of the output
// viewport's content.
func (l repl) outputRegionView() string {
	h := l.screen.Height()
	if h <= 0 {
		return ""
	}

	lines := []string{}
	if content := l.screen.GetContent(); content != "" {
		lines = strings.Split(content, "\n")
	}

	start := min(l.screen.YOffset(), len(lines))
	end := min(start+h, len(lines))
	visible := slices.Clone(lines[start:end])
	if pad := h - len(visible); pad > 0 {
		visible = append(make([]string, pad), visible...)
	}
	return strings.Join(visible, "\n")
}

// handleMouseWheel forwards wheel scroll events to the output viewport when
// alt-screen mode is active.
//
// NOTE: this case fell through to Update's shared tail (syncViewportSize)
// rather than returning early; that is reproduced explicitly here.
func (l repl) handleMouseWheel(msg tea.MouseWheelMsg) (repl, tea.Cmd) {
	var cmd tea.Cmd
	if l.altScreen {
		l.screen, cmd = l.screen.Update(msg)
	}
	return l.syncViewportSize(), cmd
}

// handleLog routes a queued log line (see logsink.go) to either the output
// buffer (alt-screen mode) or directly to the terminal via tea.Println.
func (l repl) handleLog(msg logMsg) (repl, tea.Cmd) {
	s := fmt.Sprintf(msg.template, msg.args...)
	if l.altScreen {
		l = l.appendOutput(s)
		return l, nil
	}
	return l, tea.Println(strings.TrimRight(s, "\r\n"))
}

// transcriptView renders the plain (non-alt-screen) mode: only the active
// editor is drawn; previously evaluated input/output are written directly to
// the terminal's natural scrollback via tea.Println (see pipeline.go).
func (l repl) transcriptView(cursor *tea.Cursor) tea.View {
	var v tea.View
	v.SetContent(l.edit.View().Content)
	v.Cursor = cursor
	v.AltScreen = false
	return v
}

// altScreenView renders alt-screen mode: a scrollable output region on top,
// with the active editor pinned to the bottom.
func (l repl) altScreenView(cursor *tea.Cursor) tea.View {
	var v tea.View
	editContent := l.edit.View().Content
	l = l.syncViewportSize()
	output := l.outputRegionView()
	if output != "" {
		v.SetContent(output + "\n" + editContent)
	} else {
		v.SetContent(editContent)
	}
	if cursor != nil {
		shifted := *cursor
		shifted.Y += l.screen.Height()
		cursor = &shifted
	}
	v.Cursor = cursor
	v.AltScreen = true
	return v
}
