package cli

import (
	"context"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

type repl struct {
	text textarea.Model
}

func newREPL() repl {
	t := textarea.New()
	t.ShowLineNumbers = false
	t.DynamicHeight = true
	t.MinHeight = 1
	t.MaxContentHeight = 512
	t.SetVirtualCursor(false)
	t.Focus()

	return repl{text: t}
}

func runREPL(ctx context.Context) error {
	_, err := tea.NewProgram(newREPL(), tea.WithContext(ctx)).Run()
	return err
}

func (r repl) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.RequestBackgroundColor)
}

func (r repl) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.text.SetWidth(msg.Width)
		r.text.MaxHeight = msg.Height
	case tea.BackgroundColorMsg:
		r.text.SetStyles(textarea.DefaultStyles(msg.IsDark()))
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return r, tea.Quit
		}
	}

	var cmd tea.Cmd
	r.text, cmd = r.text.Update(msg)
	return r, cmd
}

func (r repl) View() tea.View {
	var c *tea.Cursor
	if !r.text.VirtualCursor() {
		c = r.text.Cursor()
	}

	v := tea.NewView(r.text.View())
	v.Cursor = c

	return v
}
