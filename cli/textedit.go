package cli

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ardnew/aenv/log"
)

type (
	LineEdit struct{ textinput.Model }
	AreaEdit struct{ textarea.Model }
)

func flatline(value ...string) string {
	return strings.ReplaceAll(strings.Join(value, " "), "\n", " ")
}

type TextEdit struct {
	mode editMode

	size bounds[int]

	line LineEdit
	area AreaEdit

	value string
}

func (e TextEdit) activeDim() tea.Position {
	switch e.mode {
	case editLine:
		return e.line.dim()
	case editArea:
		return e.area.dim()
	}
	return tea.Position{}
}

func (e TextEdit) setFocus(mode editMode) TextEdit {
	switch mode {
	case editNone:
		log.Tracef(log.Attrs("mode", mode, "value", e.value), "blur edit")
		e.line.Blur()
		e.area.Blur()

	case editLine:
		log.Tracef(log.Attrs("mode", mode, "value", e.value), "focus edit")
		e.area.Blur()
		_ = e.line.Focus()

	case editArea:
		log.Tracef(log.Attrs("mode", mode, "value", e.value), "focus edit")
		e.line.Blur()
		_ = e.area.Focus()
	}
	return e
}

func (e TextEdit) reset() TextEdit {
	log.Tracef(log.Attrs("mode", e.mode, "value", e.value), "reset edit")
	e.line.Reset()
	e.area.Reset()
	e.value = ""
	return e
}

func (e TextEdit) setSize(size bounds[int]) TextEdit {
	e.size = size
	e.line.SetWidth(size.x.max)
	e.area.SetWidth(size.x.max)
	e.area.MinHeight = 1
	e.area.MaxHeight = min(e.area.MaxHeight, size.y.max)
	return e
}

func (e TextEdit) setBackgroundColor(_ color.Color, isDark bool) TextEdit {
	e.area.SetStyles(textarea.DefaultStyles(isDark))
	return e
}

func (e TextEdit) snapshot(cb func(string)) TextEdit {
	if cb == nil {
		return e
	}
	switch e.mode {
	case editLine:
		cb(e.line.View().Content)
	case editArea:
		cb(e.area.View().Content)
	}
	return e
}

func (e TextEdit) moveCursorEnd() TextEdit {
	var pos tea.Position
	switch e.mode {
	case editLine:
		e.line.CursorEnd()
		if c := e.line.Cursor(); c != nil {
			pos = c.Position
		}
	case editArea:
		e.area.MoveToEnd()
		if c := e.area.Cursor(); c != nil {
			pos = c.Position
		}
	}
	log.Tracef(log.Attrs("mode", e.mode, "value", e.value, "pos", pos), "move cursor to end")
	return e
}

func (e TextEdit) cursor(cb func(bool, *tea.Cursor)) TextEdit {
	if cb == nil {
		return e
	}
	switch e.mode {
	case editLine:
		cb(e.line.VirtualCursor(), e.line.Cursor())
	case editArea:
		cb(e.area.VirtualCursor(), e.area.Cursor())
	}
	return e
}

func (e TextEdit) setValue(value string) TextEdit {
	log.Tracef(log.Attrs("mode", e.mode, "old", e.value, "new", value), "set value")
	e.value = value
	e.area.SetValue(value)
	e.line.SetValue(flatline(value))
	return e
}

type editMode int

//go:generate go tool stringer -type=editMode -linecomment
const (
	editNone editMode = iota // none
	editLine                 // single-line
	editArea                 // multi-line
)

func nextEditMode(mode editMode) editMode {
	switch mode {
	case editLine:
		return editArea
	case editArea:
		return editLine
	default:
		return editArea
	}
}

func makeTextEdit() TextEdit {
	e := TextEdit{
		line:  makeLineEdit(),
		area:  makeAreaEdit(),
		value: "",
	}
	e.mode = editArea
	e = e.setFocus(editArea)
	return e
}

func makeLineEdit(value ...string) LineEdit {
	mod := textinput.New()
	mod.ShowSuggestions = false
	mod.Placeholder = ""
	mod.CharLimit = 0
	mod.Prompt = ""
	mod.SetValue(flatline(value...))
	mod.SetVirtualCursor(true)
	return LineEdit{Model: mod}
}

func (e LineEdit) dim() tea.Position {
	return tea.Position{X: e.Width(), Y: 1}
}

func makeAreaEdit(value ...string) AreaEdit {
	mod := textarea.New()
	mod.ShowLineNumbers = true
	mod.DynamicHeight = true
	mod.MinHeight = 1
	mod.MaxHeight = 8
	mod.MaxContentHeight = 512
	mod.Placeholder = ""
	mod.Prompt = ""
	mod.SetValue(strings.Join(value, "\n"))
	mod.SetVirtualCursor(true)
	return AreaEdit{Model: mod}
}

func (e AreaEdit) dim() tea.Position {
	return tea.Position{X: e.Width(), Y: e.MaxHeight}
}

func (e TextEdit) currentValue() string {
	switch e.mode {
	case editLine:
		return e.line.Value()
	case editArea:
		return e.area.Value()
	}
	return e.value
}

func (e TextEdit) isLineMode() bool { return e.mode == editLine }

func (e TextEdit) atFirstLine() bool {
	if e.isLineMode() {
		return true
	}
	return e.area.Line() == 0
}

func (e TextEdit) atLastLine() bool {
	if e.isLineMode() {
		return true
	}
	return e.area.Line() == e.area.LineCount()-1
}

func (e TextEdit) Init() tea.Cmd {
	// Initialize both models, not only the active binding.
	return tea.Batch(
		e.line.Init(),
		e.area.Init(),
	)
}

func (LineEdit) Init() tea.Cmd {
	return textinput.Blink
}

func (AreaEdit) Init() tea.Cmd {
	return textarea.Blink
}

func (e TextEdit) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch e.mode {
	case editLine:
		mod, next := e.line.Update(msg)
		cmd = next
		switch model := mod.(type) {
		case LineEdit:
			e.line = model
		case *LineEdit:
			e.line = *model
		}
	case editArea:
		mod, next := e.area.Update(msg)
		cmd = next
		switch model := mod.(type) {
		case AreaEdit:
			e.area = model
		case *AreaEdit:
			e.area = *model
		}
	}
	e.value = e.currentValue()
	return e, cmd
}

func (e LineEdit) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mod, cmd := e.Model.Update(msg)
	e.Model = mod
	return e, cmd
}

func (e AreaEdit) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mod, cmd := e.Model.Update(msg)
	e.Model = mod
	return e, cmd
}

func (e TextEdit) View() tea.View {
	switch e.mode {
	case editLine:
		return e.line.View()
	case editArea:
		return e.area.View()
	}
	return tea.View{}
}

func (e LineEdit) View() tea.View {
	return tea.NewView(e.Model.View())
}

func (e AreaEdit) View() tea.View {
	return tea.NewView(e.Model.View())
}
