package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

// editASTMsg is sent when AST editing completes successfully.
type editASTMsg struct{ ast *lang.AST }

// editCancelledMsg is sent when the user cleared the editor content.
type editCancelledMsg struct{}

// editDeclinedMsg is sent when the user declined to re-edit after a parse
// error.
type editDeclinedMsg struct{}

// editErrorMsg is sent when the edit process encounters a non-parse error.
type editErrorMsg struct{ err error }

const (
	evalPrompt = "➜ "
	ctrlPrompt = " :"
)

func helpMessage() string {
	return `
: Commands (press Esc to toggle mode):

  help     Print this cruft
  list     List top-level namespaces
  edit     Edit source in external $EDITOR
  clear    Clear screen
  quit     Exit REPL

Usage:
  Type an expression to evaluate it (namespaces are variables)
  Completions appear automatically as you type
  Press Tab / Shift-Tab to cycle through candidates
  Press Space to accept the current candidate
  Press Esc to toggle between eval and command modes
  Use Up/Down arrows for history navigation (mode switches automatically)
  Use Shift+Up/Shift+Down for history navigation within current mode only
  Use Alt+Up/Alt+Down to switch to command mode and navigate command history
    (restores original mode when reaching end of history)
  Press Ctrl+C on empty line or Ctrl+D to exit
`
}

// inputMode represents the current input mode.
type inputMode int

const (
	modeEval inputMode = iota
	modeCtrl
)

// Styles.
var (
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)
	ctrlPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")).
			Bold(true)
	inputStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	resultStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	hintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	suggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	selectedStyle   = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("4"))
)

// formatCommand formats the command echo line with prompt and input styled.
func formatCommand(input string) string {
	return promptStyle.Render(evalPrompt) + inputStyle.Render(input)
}

// formatCtrlCommand formats the control command echo line with prompt and input
// styled.
func formatCtrlCommand(input string) string {
	return ctrlPromptStyle.Render(ctrlPrompt) + inputStyle.Render(input)
}

// model is the Bubble Tea model for the REPL.
type model struct {
	ctxFunc          func() context.Context
	input            textinput.Model
	ast              *lang.AST
	logger           log.Logger
	history          *History
	historyIdx       int
	matches          fuzzy.Matches // current fuzzy match results
	candidates       []string      // backing candidate list
	wordStart        int           // byte offset of current word start
	wordEnd          int           // byte offset of current word end
	suggIdx          int           // selected candidate index
	tabActive        bool          // whether user is tab-cycling
	preTabText       string        // input text before tab-cycling began
	preTabCursor     int           // cursor position before tab-cycling began
	altNavActive     bool          // whether user is in Alt+Up/Down navigation
	altNavOrigMode   inputMode     // original mode before Alt navigation
	altNavOrigText   string        // original text before Alt navigation
	altNavOrigCursor int           // original cursor position before Alt navigation
	width            int           // terminal width for ellipsization
	quitting         bool
	mode             inputMode
	evalText         string
	evalCursor       int
	ctrlText         string
	ctrlCursor       int
}

// Run starts the REPL with the given source reader.
func Run(
	ctx context.Context,
	reader io.Reader,
	cacheDir string,
	logger log.Logger,
) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)

	defer func(err *error) { cancel(*err) }(&err)

	logger.TraceContext(
		ctx,
		"repl start",
		slog.String("cache_dir", cacheDir),
		slog.Bool("has_source", reader != nil),
	)

	if reader == nil {
		return ErrNoSource
	}

	ast, err := lang.ParseReader(
		ctx,
		reader,
		lang.WithLogger(logger), // AST gets wrapped logger
	)
	if err != nil {
		return err
	}

	logger.TraceContext(
		ctx,
		"repl ast loaded",
		slog.Int("namespace_count", len(ast.Namespaces)),
	)

	// Validate that all non-parameterized namespaces can be evaluated
	// This catches configuration errors early before starting the REPL
	if err := ast.ValidateNamespaces(ctx, lang.WithLogger(logger)); err != nil {
		return err
	}

	logger.TraceContext(
		ctx,
		"repl namespaces validated",
	)

	history := NewHistory(filepath.Join(cacheDir, baseHistory))
	if err := history.Load(); err != nil {
		fmt.Printf("Warning: could not load history: %v\n", err)
	}

	logger.TraceContext(
		ctx,
		"repl history loaded",
		slog.Int("entry_count", history.Len()),
	)

	m := newModel(ctx, ast, history, logger)

	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err = p.Run()

	return err
}

const defaultWidth = 80

func newModel(
	ctx context.Context,
	ast *lang.AST,
	history *History,
	logger log.Logger,
) model {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render(evalPrompt)
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = defaultWidth

	return model{
		ctxFunc:    func() context.Context { return ctx },
		input:      ti,
		ast:        ast,
		logger:     logger,
		history:    history,
		historyIdx: history.Len(),
		width:      defaultWidth,
		mode:       modeEval,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = msg.Width - len(evalPrompt) - 2

		return m, nil

	case editASTMsg:
		m.ast = msg.ast
		m.logger.TraceContext(
			m.ctxFunc(),
			"repl edit complete",
			slog.Int("namespace_count", len(m.ast.Namespaces)),
		)

		return m, tea.Println(resultStyle.Render("✔ — AST updated successfully"))

	case editCancelledMsg:
		return m, tea.Println(hintStyle.Render("🗴 — edit cancelled."))

	case editDeclinedMsg:
		m.quitting = true

		return m, tea.Quit

	case editErrorMsg:
		return m, tea.Println(
			errorStyle.Render("🗴 — error: " + msg.err.Error()),
		)
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Input line.
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Completion / hint line.
	input := m.input.Value()

	// Check if we're viewing history
	viewingHistory := m.historyIdx < m.history.Len()

	// Check if cursor is inside a function call
	cursor := m.input.Position()
	funcCall := detectFunctionCall(input, cursor)

	switch {
	case viewingHistory:
		// Show history position indicator
		pos := m.historyIdx + 1 // 1-based for display
		total := m.history.Len()
		hint := fmt.Sprintf("%s/%d",
			lipgloss.NewStyle().Bold(true).Render(strconv.Itoa(pos)),
			total)
		b.WriteString(hintStyle.Render(hint))
		b.WriteString("\n")

	case strings.TrimSpace(input) == "":
		// Empty or whitespace-only input: show hint.
		var hint string
		if m.mode == modeEval {
			hint = "Type an expression or press Esc for commands"
		} else {
			hint = "Type: help, list, edit, clear, quit (press Esc to return)"
		}

		b.WriteString(hintStyle.Render(hint))
		b.WriteString("\n")

	case funcCall.inCall && m.mode == modeEval:
		// Show function signature hint with current parameter highlighted
		signature, params := getSignature(m.ast, funcCall.name)
		if signature != "" {
			hint := renderSignatureHint(signature, params, funcCall.argIndex)
			b.WriteString(hint)
			b.WriteString("\n")
		} else {
			// Function not found, show completion bar if available
			if len(m.matches) > 0 {
				bar := renderCandidateBar(
					m.matches, m.suggIdx, m.tabActive, m.width,
				)
				b.WriteString(bar)
				b.WriteString("\n")
			} else {
				b.WriteString("\n")
			}
		}

	case len(m.matches) > 0:
		// Render horizontal candidate bar.
		bar := renderCandidateBar(
			m.matches, m.suggIdx, m.tabActive, m.width,
		)
		b.WriteString(bar)
		b.WriteString("\n")

	default:
		// Non-empty input but no matches: blank line.
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) handleKey(msg tea.KeyMsg) (model, tea.Cmd) {
	m.logger.TraceContext(
		m.ctxFunc(),
		"repl keypress",
		slog.String("key", msg.String()),
		slog.Int("type", int(msg.Type)),
	)

	switch msg.Type {
	case tea.KeyCtrlC:
		if m.input.Value() == "" {
			m.quitting = true

			return m, tea.Quit
		}

		m.input.SetValue("")
		m.tabActive = false
		m.altNavActive = false
		m.historyIdx = m.history.Len()
		refreshMatches(&m, false)

		return m, nil

	case tea.KeyCtrlD:
		if m.input.Value() == "" {
			m.quitting = true

			return m, tea.Quit
		}

		return m, nil

	case tea.KeyEnter:
		if !m.tabActive || len(m.matches) == 0 {
			m.altNavActive = false

			return m.executeInput()
		}
		// Lock in the current tab candidate without executing.
		m.tabActive = false
		m.altNavActive = false
		refreshMatches(&m, true)

		return m, nil

	case tea.KeyTab:
		return m.handleTab()

	case tea.KeyShiftTab:
		return m.handleShiftTab()

	case tea.KeyUp:
		if msg.Alt {
			return m.historyPrevCtrl()
		}

		return m.historyPrev()

	case tea.KeyDown:
		if msg.Alt {
			return m.historyNextCtrl()
		}

		return m.historyNext()

	case tea.KeyShiftUp:
		return m.historyPrevInMode()

	case tea.KeyShiftDown:
		return m.historyNextInMode()

	case tea.KeyEsc:
		if m.tabActive {
			m.tabActive = false
			m.input.SetValue(m.preTabText)
			m.input.SetCursor(m.preTabCursor)
			refreshMatches(&m, false)

			return m, nil
		}

		if m.altNavActive {
			m.altNavActive = false
		}

		return m.toggleMode()

	case tea.KeyRunes:
		// Check for space as "breaking" key while tab-cycling.
		if m.tabActive && msg.String() == " " {
			m.tabActive = false
		}

		var cmd tea.Cmd

		// Reset history index when typing
		m.historyIdx = m.history.Len()
		m.input, cmd = m.input.Update(msg)
		refreshMatches(&m, true)

		return m, cmd
	}

	// For any other key (backspace, delete, arrows, etc.),
	// update input and recompute matches without auto-confirm.
	var cmd tea.Cmd

	m.tabActive = false
	m.altNavActive = false
	// Reset history index when typing
	m.historyIdx = m.history.Len()
	m.input, cmd = m.input.Update(msg)
	refreshMatches(&m, false)

	return m, cmd
}

func (m model) handleTab() (model, tea.Cmd) {
	if len(m.matches) == 0 {
		return m, nil
	}

	// Single candidate: complete and confirm immediately.
	if len(m.matches) == 1 {
		replaceCurrentWord(&m, m.matches[0].Str)
		m.tabActive = false
		m.suggIdx = -1
		m.matches = nil

		return m, nil
	}

	if m.tabActive {
		// Cycle forward through candidates.
		m.suggIdx++
		if m.suggIdx >= len(m.matches) {
			m.suggIdx = 0
		}
	} else {
		m.tabActive = true
		m.preTabText = m.input.Value()
		m.preTabCursor = m.input.Position()
		m.suggIdx = 0
	}

	replaceCurrentWord(&m, m.matches[m.suggIdx].Str)

	return m, nil
}

func (m model) handleShiftTab() (model, tea.Cmd) {
	if len(m.matches) == 0 {
		return m, nil
	}

	// Single candidate: complete and confirm immediately.
	if len(m.matches) == 1 {
		replaceCurrentWord(&m, m.matches[0].Str)
		m.tabActive = false
		m.suggIdx = -1
		m.matches = nil

		return m, nil
	}

	if m.tabActive {
		// Cycle backward through candidates.
		m.suggIdx--
		if m.suggIdx < 0 {
			m.suggIdx = len(m.matches) - 1
		}
	} else {
		m.tabActive = true
		m.preTabText = m.input.Value()
		m.preTabCursor = m.input.Position()
		m.suggIdx = len(m.matches) - 1
	}

	replaceCurrentWord(&m, m.matches[m.suggIdx].Str)

	return m, nil
}

// replaceCurrentWord replaces the current word boundaries in the input with
// the given replacement text and repositions the cursor.
func replaceCurrentWord(m *model, replacement string) {
	input := m.input.Value()
	newInput := input[:m.wordStart] + replacement + input[m.wordEnd:]
	newCursor := m.wordStart + len(replacement)

	m.input.SetValue(newInput)
	m.input.SetCursor(newCursor)

	// Update word boundaries for the replaced text.
	m.wordEnd = newCursor
}

// refreshMatches recomputes fuzzy matches for the current input state.
// When autoConfirm is true it also auto-confirms the completion when exactly
// one candidate remains and the typed word already equals that candidate.
// autoConfirm should be false for deletions and cursor navigation so that
// the user can freely edit without unexpected completions.
func refreshMatches(m *model, autoConfirm bool) {
	m.matches, m.candidates, m.wordStart, m.wordEnd = m.computeMatches()

	if !m.tabActive {
		m.suggIdx = -1
	}

	if !autoConfirm || len(m.matches) != 1 {
		return
	}

	// Auto-confirm when the typed word already equals the sole candidate.
	candidate := m.matches[0].Str
	word := m.input.Value()[m.wordStart:m.wordEnd]

	if word == candidate {
		replaceCurrentWord(m, candidate)
		m.tabActive = false
		m.suggIdx = -1
		m.matches = nil
	}
}

func (m model) executeInput() (model, tea.Cmd) {
	input := strings.TrimSpace(m.input.Value())
	if input == "" {
		return m, nil
	}

	// Reset both mode inputs after submission
	m.evalText = ""
	m.evalCursor = 0
	m.ctrlText = ""
	m.ctrlCursor = 0
	m.input.SetValue("")

	if m.mode == modeCtrl {
		// Control mode - add to history and execute command
		_, _ = m.history.WriteWithMode(input, modeCtrl)
		m.historyIdx = m.history.Len()
		m.logger.TraceContext(
			m.ctxFunc(),
			"repl command",
			slog.String("input", input),
		)

		return m.executeCommand(input)
	}

	// Eval mode - add to history and evaluate
	_, _ = m.history.WriteWithMode(input, modeEval)
	m.historyIdx = m.history.Len()
	m.logger.TraceContext(
		m.ctxFunc(),
		"repl eval",
		slog.String("input", input),
	)

	// Echo the command
	echoCmd := tea.Println(formatCommand(input))

	// Evaluate
	result, err := m.ast.EvaluateExpr(m.ctxFunc(), input)
	if err != nil {
		m.logger.TraceContext(
			m.ctxFunc(),
			"repl eval result",
			slog.String("result_type", "error"),
			slog.String("error", err.Error()),
		)

		return m, tea.Sequence(
			echoCmd,
			tea.Println(errorStyle.Render("error: "+err.Error())),
		)
	}

	m.logger.TraceContext(
		m.ctxFunc(),
		"repl eval result",
		slog.String("result_type", resultTypeName(result)),
	)

	return m, tea.Sequence(
		echoCmd,
		tea.Println(resultStyle.Render(lang.FormatResult(result))),
	)
}

func (m model) executeCommand(
	input string,
) (model, tea.Cmd) {
	// Parse command and arguments
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	echoCmd := tea.Println(formatCtrlCommand(input))

	cmd := parts[0]
	args := parts[1:]

	m.logger.TraceContext(
		m.ctxFunc(),
		"repl exec command",
		slog.String("command", cmd),
		slog.Any("args", args),
	)

	switch cmd {
	case "q", "quit", "exit":
		m.quitting = true

		return m, tea.Sequence(echoCmd, tea.Quit)

	case "h", "help":
		return m, tea.Sequence(echoCmd, tea.Println(m.helpView()))

	case "l", "list":
		return m, tea.Sequence(echoCmd, tea.Println(m.listNamespaces()))

	case "c", "clear":
		return m, tea.ClearScreen

	case "e", "edit":
		var editCmd tea.Cmd

		m, editCmd = m.handleEdit(m.ctxFunc())

		return m, tea.Sequence(echoCmd, editCmd)

	default:
		return m, tea.Println(
			errorStyle.Render("Unknown command: " + cmd + " (try 'help')"),
		)
	}
}

func (m model) handleEdit(_ context.Context) (model, tea.Cmd) {
	cmd := &editASTCommand{
		ast:     m.ast,
		ctxFunc: m.ctxFunc,
		logger:  m.logger,
	}

	return m, tea.Exec(cmd, func(err error) tea.Msg {
		if errors.Is(err, ErrEditDeclined) {
			return editDeclinedMsg{}
		}

		if err != nil {
			return editErrorMsg{err: err}
		}

		if cmd.newAST == nil {
			return editCancelledMsg{}
		}

		return editASTMsg{ast: cmd.newAST}
	})
}

func (m model) historyPrev() (model, tea.Cmd) {
	if m.historyIdx > 0 {
		m.historyIdx--

		if entry, err := m.history.GetEntry(m.historyIdx); err == nil {
			// Switch mode if needed
			if m.mode != entry.Mode {
				m, _ = m.switchToMode(entry.Mode)
			}

			m.input.SetValue(entry.Line)
			m.input.SetCursor(len(entry.Line))
			refreshMatches(&m, false)
		}
	}

	return m, nil
}

func (m model) historyNext() (model, tea.Cmd) {
	if m.historyIdx < m.history.Len()-1 {
		m.historyIdx++

		if entry, err := m.history.GetEntry(m.historyIdx); err == nil {
			// Switch mode if needed
			if m.mode != entry.Mode {
				m, _ = m.switchToMode(entry.Mode)
			}

			m.input.SetValue(entry.Line)
			m.input.SetCursor(len(entry.Line))
			refreshMatches(&m, false)
		}
	} else {
		m.historyIdx = m.history.Len()
		m.input.SetValue("")
		refreshMatches(&m, false)
	}

	return m, nil
}

func (m model) historyPrevInMode() (model, tea.Cmd) {
	currentMode := m.mode

	for i := m.historyIdx - 1; i >= 0; i-- {
		if entry, err := m.history.GetEntry(i); err == nil {
			if entry.Mode == currentMode {
				m.historyIdx = i
				m.input.SetValue(entry.Line)
				m.input.SetCursor(len(entry.Line))
				refreshMatches(&m, false)

				return m, nil
			}
		}
	}

	return m, nil
}

func (m model) historyNextInMode() (model, tea.Cmd) {
	currentMode := m.mode

	for i := m.historyIdx + 1; i < m.history.Len(); i++ {
		if entry, err := m.history.GetEntry(i); err == nil {
			if entry.Mode == currentMode {
				m.historyIdx = i
				m.input.SetValue(entry.Line)
				m.input.SetCursor(len(entry.Line))
				refreshMatches(&m, false)

				return m, nil
			}
		}
	}

	// Reached end of mode-specific history, clear input
	if m.historyIdx < m.history.Len() {
		m.historyIdx = m.history.Len()
		m.input.SetValue("")
		refreshMatches(&m, false)
	}

	return m, nil
}

func (m model) historyPrevCtrl() (model, tea.Cmd) {
	// Save original state on first Alt navigation
	if !m.altNavActive {
		m.altNavActive = true
		m.altNavOrigMode = m.mode
		m.altNavOrigText = m.input.Value()
		m.altNavOrigCursor = m.input.Position()

		// Switch to command mode if not already there
		if m.mode != modeCtrl {
			m, _ = m.switchToMode(modeCtrl)
		}
	}

	for i := m.historyIdx - 1; i >= 0; i-- {
		if entry, err := m.history.GetEntry(i); err == nil {
			if entry.Mode == modeCtrl {
				m.historyIdx = i
				m.input.SetValue(entry.Line)
				m.input.SetCursor(len(entry.Line))
				refreshMatches(&m, false)

				return m, nil
			}
		}
	}

	// Reached start of ctrl history - restore original state
	if m.altNavActive {
		m.altNavActive = false
		if m.altNavOrigMode != m.mode {
			m, _ = m.switchToMode(m.altNavOrigMode)
		}

		m.input.SetValue(m.altNavOrigText)
		m.input.SetCursor(m.altNavOrigCursor)
		m.historyIdx = m.history.Len()
		refreshMatches(&m, false)
	}

	return m, nil
}

func (m model) historyNextCtrl() (model, tea.Cmd) {
	// Save original state on first Alt navigation
	if !m.altNavActive {
		m.altNavActive = true
		m.altNavOrigMode = m.mode
		m.altNavOrigText = m.input.Value()
		m.altNavOrigCursor = m.input.Position()

		// Switch to command mode if not already there
		if m.mode != modeCtrl {
			m, _ = m.switchToMode(modeCtrl)
		}
	}

	for i := m.historyIdx + 1; i < m.history.Len(); i++ {
		if entry, err := m.history.GetEntry(i); err == nil {
			if entry.Mode == modeCtrl {
				m.historyIdx = i
				m.input.SetValue(entry.Line)
				m.input.SetCursor(len(entry.Line))
				refreshMatches(&m, false)

				return m, nil
			}
		}
	}

	// Reached end of ctrl history - restore original state
	if m.altNavActive {
		m.altNavActive = false
		if m.altNavOrigMode != m.mode {
			m, _ = m.switchToMode(m.altNavOrigMode)
		}

		m.input.SetValue(m.altNavOrigText)
		m.input.SetCursor(m.altNavOrigCursor)
		m.historyIdx = m.history.Len()
		refreshMatches(&m, false)
	}

	return m, nil
}

func (m model) helpView() string { return helpMessage() }

func (m model) listNamespaces() string {
	var b strings.Builder

	for ns := range m.ast.All() {
		name := ns.Name
		preview := formatPreview(ns)
		b.WriteString(fmt.Sprintf("  %s %s\n", name, hintStyle.Render(preview)))
	}

	return b.String()
}

// toggleMode switches between eval and control modes, preserving input state.
func (m model) toggleMode() (model, tea.Cmd) {
	// Save current mode's input
	if m.mode == modeEval {
		m.evalText = m.input.Value()
		m.evalCursor = m.input.Position()
	} else {
		m.ctrlText = m.input.Value()
		m.ctrlCursor = m.input.Position()
	}

	// Toggle mode
	if m.mode == modeEval {
		return m.switchToMode(modeCtrl)
	}

	return m.switchToMode(modeEval)
}

// switchToMode switches to the specified mode, preserving input state.
func (m model) switchToMode(mode inputMode) (model, tea.Cmd) {
	// Save current mode's input
	if m.mode == modeEval {
		m.evalText = m.input.Value()
		m.evalCursor = m.input.Position()
	} else {
		m.ctrlText = m.input.Value()
		m.ctrlCursor = m.input.Position()
	}

	// Switch to target mode
	m.mode = mode
	if mode == modeEval {
		m.input.Prompt = promptStyle.Render(evalPrompt)
		m.input.SetValue(m.evalText)
		m.input.SetCursor(m.evalCursor)
	} else {
		m.input.Prompt = ctrlPromptStyle.Render(ctrlPrompt)
		m.input.SetValue(m.ctrlText)
		m.input.SetCursor(m.ctrlCursor)
	}

	refreshMatches(&m, false)

	return m, nil
}

func resultTypeName(value any) string {
	if value == nil {
		return "nil"
	}

	return fmt.Sprintf("%T", value)
}
