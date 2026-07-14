package cli

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/mattn/go-runewidth"

	"github.com/ardnew/aenv/log"
)

const (
	maxLinesVisible = 8
	maxLines        = 4096
)

type (
	LineEdit struct {
		textinput.Model

		promptSymbol string
	}
	AreaEdit struct {
		textarea.Model

		focusHeight int
	}
)

type TextEdit struct {
	mode editMode

	pos tea.Position

	bounds tea.Position // size of terminal

	line LineEdit
	area AreaEdit

	style struct {
		isDark bool
	}

	content string
}

func (e TextEdit) cursorPos() tea.Position {
	switch e.mode {
	case editLine:
		return tea.Position{X: e.line.Position(), Y: 0}
	case editArea:
		return tea.Position{X: e.area.Column(), Y: e.area.Line()}
	}
	return e.pos
}

func areaPosIn(value string, index int) tea.Position {
	lines := splitInput(value)
	index = clamp(index, 0, runeCount(value))
	for line, text := range lines {
		width := runeCount(text)
		if index <= width {
			return tea.Position{X: index, Y: line}
		}
		index -= width
		if index == 0 {
			return tea.Position{X: width, Y: line}
		}
		index--
	}
	last := len(lines) - 1
	return tea.Position{X: runeCount(lines[last]), Y: last}
}

func (e TextEdit) setMode(mode editMode) TextEdit {
	log.Trace(log.Attrs("mode", e.mode, "next", mode))

	pos := e.cursorPos()
	content := e.content
	if content == "" {
		content = e.value()
	}
	switch mode {
	case editLine:
		if e.mode == editArea {
			lines := splitInput(content)
			line := clamp(pos.Y, 0, len(lines)-1)
			index := 0
			for i := 0; i < line; i++ {
				index += runeCount(lines[i]) + 1
			}
			pos = tea.Position{X: index + clamp(pos.X, 0, runeCount(lines[line])), Y: 0}
		}
		e.line.SetCursor(pos.X)

	case editArea:
		if e.mode == editLine {
			pos = areaPosIn(content, pos.X)
		}
		e.area.MoveToBegin()
		for i := 0; i < pos.Y; i++ {
			e.area.CursorDown()
		}
		e.area.SetCursorColumn(pos.X)
	}

	e.mode = mode
	e.pos = pos
	return e
}

func (e TextEdit) size() tea.Position {
	switch e.mode {
	case editLine:
		return e.line.size()
	case editArea:
		return e.area.size()
	}
	return tea.Position{}
}

func (e TextEdit) setFocus(mode editMode) (TextEdit, tea.Cmd) {
	var focus tea.Cmd

	e.line.SetWidth(e.bounds.X)
	e.area.SetWidth(e.bounds.X)

	switch mode {
	case editNone:
		log.Tracef(log.Attrs("mode", mode, "bounds", e.bounds), "blur edit")
		e.line.Blur()
		e.area.Blur()
		e.area.MaxHeight = 0
		e.area.ShowLineNumbers = false
		e.line.Prompt = ""

	case editLine:
		log.Tracef(log.Attrs("mode", mode, "bounds", e.bounds), "focus edit")
		e.area.Blur()
		focus = e.line.Focus()
		e.line.Prompt = e.line.promptSymbol

	case editArea:
		log.Tracef(log.Attrs("mode", mode, "bounds", e.bounds), "focus edit")
		e.line.Blur()
		focus = e.area.Focus()
		e.area.MaxHeight = e.area.focusHeight
		e.area.ShowLineNumbers = true
	}

	return e, focus
}

func (e TextEdit) reset() TextEdit {
	log.Tracef(log.Attrs("mode", e.mode), "reset edit")
	e.line.Reset()
	e.area.Reset()
	e.content = ""
	return e
}

func (e TextEdit) setSize(size tea.Position) TextEdit {
	log.Trace(log.Attrs("size", size))
	e.bounds = size
	e.line.SetWidth(size.X)
	e.area.SetWidth(size.X)
	e.area.MinHeight = 1
	e.area.MaxHeight = min(e.area.MaxHeight, size.Y)
	e.area.focusHeight = e.area.MaxHeight
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
	log.Tracef(log.Attrs("mode", e.mode, "pos", pos), "move cursor to end")
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
	pos := e.cursorPos()
	log.Tracef(
		log.Attrs(
			"mode", e.mode,
			"len-old", len(e.content),
			"len-new", len(value),
		),
		"set value",
	)
	e.content = value
	e.area.SetValue(value)
	e.line.SetValue(joinInput(value))
	e.pos = pos
	switch e.mode {
	case editLine:
		e.line.SetCursor(clamp(pos.X, 0, runeCount(e.line.Value())))
	case editArea:
		lines := splitInput(e.area.Value())
		line := clamp(pos.Y, 0, len(lines)-1)
		e.area.MoveToBegin()
		for i := 0; i < line; i++ {
			e.area.CursorDown()
		}
		e.area.SetCursorColumn(clamp(pos.X, 0, runeCount(lines[line])))
	}
	return e
}

type editMode int

//go:generate go tool stringer -type=editMode -linecomment
const (
	editNone editMode = iota // none
	editLine                 // single-line
	editArea                 // multi-line
)

func (m editMode) next() editMode {
	switch m {
	case editLine:
		return editArea
	case editArea:
		return editLine
	default:
		return editLine
	}
}

func makeTextEdit() TextEdit {
	e := TextEdit{
		line:    makeLineEdit().setPromptSymbol(defaultStyle(true).prompt),
		area:    makeAreaEdit(),
		content: "",
	}
	e.mode = e.mode.next()
	return e
}

func makeLineEdit(value ...string) LineEdit {
	mod := textinput.New()
	mod.ShowSuggestions = false
	mod.Prompt = ""
	mod.SetValue(joinInput(value...))
	mod.SetVirtualCursor(true)
	return LineEdit{Model: mod}
}

func (e LineEdit) size() tea.Position {
	return tea.Position{X: e.Width(), Y: 1}
}

func (e LineEdit) setPromptSymbol(symbol rune) LineEdit {
	e.promptSymbol = " " + string(symbol) + " "
	if runewidth.RuneWidth(symbol) > 1 {
		e.promptSymbol = e.promptSymbol[1:4]
	}
	return e
}

func makeAreaEdit(value ...string) AreaEdit {
	mod := textarea.New()
	mod.ShowLineNumbers = false // only visible when focused
	mod.DynamicHeight = true
	mod.MaxHeight = maxLinesVisible
	mod.MaxContentHeight = maxLines
	mod.Prompt = ""
	mod.SetValue(strings.Join(value, "\n"))
	mod.SetVirtualCursor(true)
	return AreaEdit{Model: mod}
}

func (e AreaEdit) size() tea.Position {
	return tea.Position{X: e.Width(), Y: e.MaxHeight}
}

func (e TextEdit) value() string {
	switch e.mode {
	case editLine:
		return e.line.Value()
	case editArea:
		return e.area.Value()
	}
	return e.content
}

func (e TextEdit) atFirstLine() bool {
	return e.mode != editArea || e.area.Line() == 0
}

func (e TextEdit) atLastLine() bool {
	return e.mode != editArea || e.area.Line() == e.area.LineCount()-1
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
	var cmd []tea.Cmd
	switch e.mode {
	case editLine:
		mod, next := e.line.Update(msg)
		cmd = append(cmd, next)
		switch model := mod.(type) {
		case LineEdit:
			e.line = model
		case *LineEdit:
			e.line = *model
		}
	case editArea:
		lineCountBefore := e.area.LineCount()
		mod, next := e.area.Update(msg)
		cmd = append(cmd, next)
		switch model := mod.(type) {
		case AreaEdit:
			e.area = model
		case *AreaEdit:
			e.area = *model
		}
		if lineCountBefore > 1 && e.area.LineCount() == 1 {
			// Allow the area edit to process the keypress before toggling mode.
			cmd = append(cmd, setEditMode(editLine))
		}
	}
	e.content = e.value()
	return e, tea.Batch(cmd...)
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
