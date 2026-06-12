package repl

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"

	tea "github.com/charmbracelet/bubbletea"
)

type textEditMode int

const (
	editArea textEditMode = iota
	editLine
)

type TextEdit struct {
	mode   textEditMode
	area   textarea.Model
	line   textinput.Model
	prompt string
	value  string
}

func makeTextEdit(prompt string, width int) TextEdit {
	area := textarea.New()
	area.Prompt = prompt
	area.CharLimit = 1024
	area.SetWidth(width)
	area.Focus()

	line := textinput.New()
	line.Prompt = prompt
	line.CharLimit = 1024
	line.Width = width
	line.Focus()

	return TextEdit{
		mode:   editArea,
		area:   area,
		line:   line,
		prompt: prompt,
		value:  "",
	}
}

func (e TextEdit) Init() tea.Cmd {
	return nil
}

func (e TextEdit) Update(msg tea.Msg) (TextEdit, tea.Cmd) {
	var cmd tea.Cmd

	if e.mode == editArea {
		e.area, cmd = e.area.Update(msg)
		e.syncFromArea()
	} else {
		e.line, cmd = e.line.Update(msg)
		e.syncFromLine()
	}

	return e, cmd
}

func (e TextEdit) View() string {
	if e.mode == editArea {
		return e.area.View()
	}

	return e.line.View()
}

func (e TextEdit) Mode() textEditMode {
	return e.mode
}

func (e *TextEdit) ToggleMode() {
	if e.mode == editArea {
		e.mode = editLine
	} else {
		e.mode = editArea
	}
}

func (e *TextEdit) SetPrompt(prompt string) {
	e.prompt = prompt
	e.area.Prompt = prompt
	e.line.Prompt = prompt
}

func (e TextEdit) Value() string {
	return e.value
}

func (e *TextEdit) SetValue(s string) {
	e.value = s
	e.line.SetValue(s)
	e.area.SetValue(s)

	pos := len(s)
	e.line.SetCursor(clamp(pos, 0, len(e.line.Value())))
	setAreaCursorByOffset(&e.area, pos)
}

func (e *TextEdit) Reset() {
	e.SetValue("")
}

func (e TextEdit) Position() int {
	if e.mode == editArea {
		return areaCursorOffset(e.area)
	}

	return e.line.Position()
}

func (e *TextEdit) SetCursor(pos int) {
	if pos < 0 {
		pos = 0
	}

	if pos > len(e.Value()) {
		pos = len(e.Value())
	}

	e.line.SetCursor(clamp(pos, 0, len(e.line.Value())))
	setAreaCursorByOffset(&e.area, pos)
}

func (e *TextEdit) SetWidth(width int) {
	e.line.Width = width
	e.area.SetWidth(width)
}

func (e *TextEdit) Focus() tea.Cmd {
	cmdLine := e.line.Focus()
	cmdArea := e.area.Focus()

	return tea.Batch(cmdLine, cmdArea)
}

func (e *TextEdit) Blur() {
	e.line.Blur()
	e.area.Blur()
}

func (e *TextEdit) syncFromArea() {
	value := e.area.Value()
	pos := areaCursorOffset(e.area)
	e.value = value
	e.line.SetValue(value)
	e.line.SetCursor(clamp(pos, 0, len(e.line.Value())))
}

func (e *TextEdit) syncFromLine() {
	value := e.line.Value()
	pos := e.line.Position()
	e.value = value
	e.area.SetValue(value)
	setAreaCursorByOffset(&e.area, pos)
}

func areaCursorOffset(m textarea.Model) int {
	value := m.Value()
	if value == "" {
		return 0
	}

	lines := strings.Split(value, "\n")
	line := m.Line()
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}

	offset := 0
	for i := 0; i < line; i++ {
		offset += len(lines[i]) + 1
	}

	info := m.LineInfo()
	col := clamp(info.CharOffset, 0, utf8.RuneCountInString(lines[line]))
	offset += len(string([]rune(lines[line])[:col]))

	if offset > len(value) {
		return len(value)
	}

	return offset
}

func setAreaCursorByOffset(m *textarea.Model, pos int) {
	value := m.Value()
	pos = clamp(pos, 0, len(value))
	line, col := byteOffsetToLineCol(value, pos)
	m.CursorStart()
	for i := 0; i < line; i++ {
		m.CursorDown()
	}

	m.SetCursor(col)
}

func byteOffsetToLineCol(s string, offset int) (line int, col int) {
	offset = clamp(offset, 0, len(s))
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return 0, 0
	}

	for i, part := range lines {
		if offset <= len(part) {
			return i, utf8.RuneCountInString(part[:offset])
		}

		offset -= len(part)
		if i == len(lines)-1 {
			return i, utf8.RuneCountInString(part)
		}

		if offset == 0 {
			return i, utf8.RuneCountInString(part)
		}

		offset--
	}

	last := len(lines) - 1

	return last, utf8.RuneCountInString(lines[last])
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}

	if value > maxValue {
		return maxValue
	}

	return value
}
