package cli

import (
	"image/color"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// This file holds everything about the TextEdit editors' presentation
// (colors/styles), separate from textedit.go's mode-dispatch/model logic.

type editStyle struct {
	prompt rune
	editor lipgloss.Style
	border lipgloss.Style
	cursor lipgloss.Style
	dimmed lipgloss.Style
	record lipgloss.Style
}

func defaultStyle(isDark bool) editStyle {
	auto := lipgloss.LightDark(isDark)
	autoColor := func(light, dark string) color.Color {
		return auto(lipgloss.Color(light), lipgloss.Color(dark))
	}

	promptSymbol := '❯'

	editorText := autoColor("#1f4f58", "#addfda")
	editorBackground := autoColor("#edf2f6", "#212324")

	cursorText := autoColor("#1f4f58", "#addfda")
	cursorBackground := autoColor("#f4f8fb", "#24292e")

	dimmedText := autoColor("#8f97a3", "#505050")

	recordText := autoColor("#778391", "#a6b3bf")

	return editStyle{
		prompt: promptSymbol,
		editor: lipgloss.NewStyle().
			Foreground(editorText).
			Background(editorBackground),
		border: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(cursorText),
		cursor: lipgloss.NewStyle().
			Foreground(cursorText).
			Background(cursorBackground),
		dimmed: lipgloss.NewStyle().
			Foreground(dimmedText).
			UnsetBackground(),
		record: lipgloss.NewStyle().
			Foreground(recordText).
			UnsetBackground(),
	}
}

func (e TextEdit) setStyle(isDark bool) TextEdit {
	e.style.isDark = isDark

	e.line = e.line.setStyle(isDark)
	e.area = e.area.setStyle(isDark)
	return e
}

func (e LineEdit) setStyle(isDark bool) LineEdit {
	st := textinput.DefaultStyles(isDark)
	et := defaultStyle(isDark)

	st.Focused.Text = et.cursor.Inherit(et.editor)
	st.Focused.Prompt = et.cursor.Inherit(et.border)
	st.Focused.Placeholder = et.dimmed.Inherit(et.editor)
	st.Focused.Suggestion = et.dimmed.Inherit(et.editor)

	st.Blurred.Text = et.record
	st.Blurred.Prompt = et.record
	st.Blurred.Placeholder = et.record
	st.Blurred.Suggestion = et.record

	e.SetStyles(st)
	return e.setPromptSymbol(et.prompt)
}

func (e AreaEdit) setStyle(isDark bool) AreaEdit {
	st := textarea.DefaultStyles(isDark)
	et := defaultStyle(isDark)

	st.Focused.Base = et.editor.Inherit(et.border)
	st.Focused.Text = et.editor
	st.Focused.LineNumber = et.editor
	st.Focused.CursorLineNumber = et.cursor.Inherit(et.editor)
	st.Focused.CursorLine = et.cursor.Inherit(et.editor)
	st.Focused.EndOfBuffer = et.editor
	st.Focused.Placeholder = et.dimmed.Inherit(et.editor)
	st.Focused.Prompt = et.cursor.Inherit(et.editor)

	st.Blurred.Base = et.record
	st.Blurred.Text = et.record
	st.Blurred.LineNumber = et.record
	st.Blurred.CursorLineNumber = et.record
	st.Blurred.CursorLine = et.record
	st.Blurred.EndOfBuffer = et.record
	st.Blurred.Placeholder = et.record
	st.Blurred.Prompt = et.record

	e.SetStyles(st)
	return e
}
