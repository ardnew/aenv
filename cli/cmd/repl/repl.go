package repl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"
)

// teaWriter is an [io.Writer] that routes output through a [tea.Program].
// Before the program is set, writes fall back to the underlying writer.
//
// Log lines are sent asynchronously via [tea.Program.Send] in a goroutine to
// avoid deadlocking the event loop when logging occurs inside [Model.Update].
type teaWriter struct {
	mu       sync.Mutex
	program  *tea.Program
	fallback io.Writer
}

// Write implements [io.Writer]. Each call is treated as one or more complete
// log lines and printed above the TUI via [tea.Program.Send].
func (w *teaWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	prog := w.program
	w.mu.Unlock()

	if prog == nil {
		return w.fallback.Write(p)
	}

	for line := range bytes.Lines(p) {
		line = bytes.TrimRight(line, "\n")
		if len(line) > 0 {
			msg := tea.Println(string(line))()
			go prog.Send(msg)
		}
	}

	return len(p), nil
}

func (w *teaWriter) setProgram(p *tea.Program) {
	w.mu.Lock()
	w.program = p
	w.mu.Unlock()
}

// editASTMsg is sent when AST editing completes successfully.
type editASTMsg struct {
	ast       *lang.AST
	unchanged bool
}

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

// helpTopic defines a single help topic with its short summary and detailed
// content.
type (
	helpTopic struct {
		name    string      // topic identifier (used in :help <topic>)
		summary string      // one-line summary shown in :help overview
		detail  helpSection // detailed help text shown by :help <topic>
	}
	helpTopics []helpTopic
	helpDetail struct {
		styled     bool
		key, value []string
	}
	helpDetails []helpDetail
	helpSection struct {
		pretty  bool
		title   string
		content helpDetails
	}
)

type (
	slicePair[T any]  [2][]T
	slicePairs[T any] []slicePair[T]
)

func renderDetail(
	s slicePair[string],
	style slicePair[lipgloss.Style],
) slicePair[string] {
	result := slicePair[string]{}
	for i := range s {
		result[i] = make([]string, len(s[i]))
		copy(result[i], s[i])

		for j := range result[i] {
			for k := range style[i] {
				result[i][j] = style[i][k].Render(result[i][j])
			}
		}
	}

	return result
}

func makeHelpSection(
	title string,
	pairs slicePairs[string],
	styles ...slicePair[lipgloss.Style],
) helpSection {
	content := make(helpDetails, len(pairs))
	for i, p := range pairs {
		content[i] = helpDetail{
			styled: i < len(styles),
			key:    p[0],
			value:  p[1],
		}
		if content[i].styled {
			details := renderDetail(p, styles[i])
			content[i].key = details[0]
			content[i].value = details[1]
		}
	}

	return helpSection{
		title:   title,
		content: content,
	}
}

func (h helpSection) String() string {
	titleStyle := lipgloss.NewStyle().Bold(h.pretty)
	sectionStyle := lipgloss.NewStyle().Bold(h.pretty).Underline(h.pretty)
	keyStyle := lipgloss.NewStyle().Bold(h.pretty)
	descStyle := lipgloss.NewStyle()

	if h.pretty {
		descStyle = descStyle.Foreground(lipgloss.Color("8"))
	}

	var b strings.Builder

	fmt.Fprintf(&b, " %s\n\n", titleStyle.Render(h.title))

	// Calculate max key width for alignment.
	maxKeyWidth := 0

	for _, item := range h.content {
		key := strings.Join(item.key, " ")
		if w := lipgloss.Width(key); w > maxKeyWidth {
			maxKeyWidth = w
		}
	}

	const (
		indent = "    "
		gap    = "   "
	)

	for _, item := range h.content {
		key := strings.Join(item.key, " ")
		values := item.value
		isSection := key != "" && len(values) == 1 && values[0] == ""
		isSpacer := key == "" && len(values) == 1 && values[0] == ""

		if isSpacer {
			b.WriteString("\n")

			continue
		}

		if isSection {
			b.WriteString(indent + sectionStyle.Render(key) + "\n")

			continue
		}

		stylize := func(s string, style lipgloss.Style) string {
			if !item.styled {
				return style.Render(s)
			}

			return s
		}

		styledKey := stylize(key, keyStyle)

		pad := strings.Repeat(" ", maxKeyWidth-lipgloss.Width(key))

		for i, val := range values {
			if i == 0 {
				fmt.Fprintf(&b, "%s%s%s%s%s\n",
					indent, styledKey, pad, gap, stylize(val, descStyle))
			} else {
				fmt.Fprintf(
					&b,
					"%s%s%s%s\n",
					indent,
					strings.Repeat(" ", maxKeyWidth),
					gap,
					stylize(val, descStyle),
				)
			}
		}
	}

	return b.String()
}

// globalHelpTopics defines the available help topics in display order.
var globalHelpTopics = helpTopics{
	{
		name:    "keys",
		summary: "Keyboard shortcuts and keybindings",
		detail: makeHelpSection(
			"Keybindings",
			slicePairs[string]{
				{{"Esc"}, {"Toggle evaluation or command mode"}},
				{{"Tab"}, {"Cycle completions forward"}},
				{{"Shift+Tab"}, {"Cycle completions backward"}},
				{{"↑/↓"}, {"Navigate all history", "· mode follows entry"}},
				{{"Shift+↑/↓"}, {"Navigate history within current mode only"}},
				{
					{"Alt+↑/↓"},
					{
						"Navigate command history",
						"· restore previous mode+input  ",
					},
				},
				{{"Ctrl+L"}, {"Clear screen"}},
				{{"Ctrl+C"}, {"Clear input", "· quit if input is empty"}},
				{{"Ctrl+D"}, {"Quit", "· discards input and signals EOF"}},
			},
		),
	},
	{
		name:    "commands",
		summary: "Available REPL commands",
		detail: makeHelpSection(
			"Commands",
			slicePairs[string]{
				{{"help"}, {"Show help overview and available topics"}},
				{{"help", "<topic>"}, {"Show detailed help for a specific topic"}},
				{{"list"}, {"List all top-level namespaces"}},
				{
					{"edit"},
					{
						"Open all sources merged in $EDITOR",
						"· recompile and reload on save, or",
						"· resume editing on parse error",
					},
				},
				{{"clear"}, {"Clear the screen"}},
				{{"quit"}, {"Exit the REPL"}},
			},
		),
	},
	{
		name:    "eval",
		summary: "Expression evaluation and syntax",
		detail: makeHelpSection(
			"Evaluation",
			slicePairs[string]{
				{{"Syntax"}, {""}},
				{{"name : expr"}, {"Define a namespace with an expression value"}},
				{{"name : { ... }"}, {"Define a block (entries separated by ;)"}},
				{{"fn a b : expr"}, {"Parameterized namespace (callable as fn(a, b))"}},
				{{"fn ...xs : expr"}, {"Variadic parameter (collects remaining args)"}},
				{{"# or //"}, {"Line comment"}},
				{{"/* ... */"}, {"Block comment"}},
				{
					{""},
					{""},
				},
				{{"Values"}, {""}},
				{
					{"42, 3.14, 0xff"},
					{"Number literals (int, float, hex, octal, binary)"},
				},
				{{"\"s\", 's', `s`"}, {"String literals (double, single, backtick)"}},
				{{"true, false, nil"}, {"Boolean and nil literals"}},
				{{"[1, 2, 3]"}, {"Array literal"}},
				{{`{"k": "v"}`}, {"Map literal"}},
				{
					{""},
					{""},
				},
				{{"Operators"}, {""}},
				{{"+ - * / %"}, {"Arithmetic"}},
				{{"== != < <= > >="}, {"Comparison"}},
				{{"and or not (&&, ||, !)"}, {"Logical"}},
				{{"x ? y : z"}, {"Ternary conditional"}},
				{{"v in arr"}, {"Membership test"}},
				{{"a[0], a[1:3]"}, {"Indexing and slicing"}},
				{{"m.key, m[\"key\"]"}, {"Member access"}},
				{
					{""},
					{""},
				},
				{{"Builtins"}, {""}},
				{
					{"target.os, target.arch"},
					{"Host target (GNU naming: x86_64, aarch64)"},
				},
				{
					{"platform.os, platform.arch"},
					{"Host platform (Go naming: amd64, arm64)"},
				},
				{{"hostname"}, {"System hostname"}},
				{
					{"user.username, user.homeDir"},
					{"Current user info (.name, .uid, .gid)"},
				},
				{{"shell"}, {"Current shell path"}},
				{{"env.HOME, env[\"PATH\"]"}, {"Process environment variables"}},
				{
					{""},
					{""},
				},
				{{"Filesystem"}, {""}},
				{{"fs.cwd()"}, {"Current working directory"}},
				{{"fs.abs(path)"}, {"Absolute path"}},
				{{"fs.cat(p1, p2, ...)"}, {"Join path segments"}},
				{{"fs.rel(from, to)"}, {"Relative path"}},
				{{"fs.stat(path)"}, {"File info (.name, .size, .mode, .perms, .type)"}},
				{
					{""},
					{""},
				},
				{{"PATH Helpers"}, {""}},
				{{"mung(key, sep, ...pfx)"}, {"Prepend to a path-like variable"}},
				{{"mungif(key, sep, pred, ...pfx)"}, {"Conditional prepend"}},
				{
					{""},
					{""},
				},
				{{"Common Functions"}, {""}},
				{{"len, string, int, float"}, {"Length, type conversion"}},
				{{"upper, lower, trim"}, {"String case and whitespace"}},
				{{"split, join, replace"}, {"String manipulation"}},
				{{"contains, startsWith, endsWith"}, {"String predicates"}},
				{{"sort, reverse, unique, flatten"}, {"Array operations"}},
				{{"map, filter, count, sum"}, {"Array higher-order functions"}},
				{{"min, max, abs, ceil, floor"}, {"Math functions"}},
				{{"keys, values"}, {"Map introspection"}},
				{{"toJSON, fromJSON"}, {"JSON conversion"}},
				{{"sprintf(fmt, args...)"}, {"Formatted string output"}},
				{
					{""},
					{""},
				},
				{{"Scoping"}, {""}},
				{{"(inner → outer)"}, {"Params > block locals > top-level > builtins"}},
				{{"block entries"}, {"Forward references not allowed within blocks"}},
				{{"duplicate names"}, {"Blocks merge, expressions: last wins"}},
			},
		),
	},
	{
		name:    "modes",
		summary: "Evaluation and command modes",
		detail: makeHelpSection(
			"Modes",
			slicePairs[string]{
				{{"➜ eval"}, {"Evaluate expressios"}},
				{{" :command"}, {"Command and control the REPL"}},
			},
			slicePair[lipgloss.Style]{
				{promptStyle}, {hintStyle},
			},
			slicePair[lipgloss.Style]{
				{ctrlPromptStyle}, {hintStyle},
			},
		),
	},
}

// helpTopicNames returns the sorted list of topic names for completion.
func helpTopicNames() []string {
	names := make([]string, len(globalHelpTopics))
	for i, t := range globalHelpTopics {
		names[i] = t.name
	}

	return names
}

// helpOverview returns the top-level help output listing available topics.
func helpOverview(pretty bool) string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(pretty)
	nameStyle := lipgloss.NewStyle().Bold(pretty)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Topics"))
	b.WriteString("\n\n")

	// Find max topic name length for alignment.
	maxLen := 0
	for _, t := range globalHelpTopics {
		if len(t.name) > maxLen {
			maxLen = len(t.name)
		}
	}

	for _, t := range globalHelpTopics {
		pad := strings.Repeat(" ", maxLen-len(t.name)+2)
		if pretty {
			fmt.Fprintf(&b, "  %s%s%s\n",
				nameStyle.Render(t.name), pad, descStyle.Render(t.summary))
		} else {
			fmt.Fprintf(&b, "  %s%s%s\n", t.name, pad, t.summary)
		}
	}

	return b.String()
}

// section returns the detailed text for a specific topic, or an error
// message if the topic is not found.
func (h helpTopics) section(topic string, pretty bool) string {
	topic = strings.ToLower(strings.TrimSpace(topic))

	for _, t := range globalHelpTopics {
		if t.name == topic {
			d := t.detail
			d.pretty = pretty

			return "\n" + d.String()
		}
	}

	var b strings.Builder

	log.Make(&b, log.WithFormat(log.FormatText), log.WithTimeLayout("")).
		Error("unknown help topic", slog.String("topic", topic))

	b.WriteString(helpOverview(pretty))

	return b.String()
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
	pretty           bool
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
	editing          bool
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
	pretty bool,
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
		reader = strings.NewReader("")
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
		logger.WarnContext(ctx, "could not load history",
			slog.String("error", err.Error()),
		)
	}

	logger.TraceContext(
		ctx,
		"repl history loaded",
		slog.Int("entry_count", history.Len()),
	)

	tw := &teaWriter{fallback: io.Discard}
	logger = logger.Wrap(log.WithOutput(tw))

	m := newModel(ctx, ast, history, logger, pretty)

	p := tea.NewProgram(m, tea.WithContext(ctx))
	tw.setProgram(p)

	_, err = p.Run()

	return err
}

const defaultWidth = 80

func newModel(
	ctx context.Context,
	ast *lang.AST,
	history *History,
	logger log.Logger,
	pretty bool,
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
		pretty:     pretty,
		history:    history,
		historyIdx: history.Len(),
		width:      defaultWidth,
		mode:       modeEval,
	}
}

func (m model) Init() tea.Cmd {
	banner := resultStyle.Render(
		fmt.Sprintf("%s version %s", pkg.Name, strings.TrimSpace(pkg.Version)),
	)

	return tea.Batch(textinput.Blink, tea.Println(banner))
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
		m.editing = false

		if msg.unchanged {
			m.logger.TraceContext(
				m.ctxFunc(),
				"repl edit unchanged",
			)

			return m, tea.Println(hintStyle.Render("✔ — AST unmodified"))
		}

		m.ast = msg.ast
		// Clear program cache since AST changed
		lang.ClearProgramCache()
		m.logger.TraceContext(
			m.ctxFunc(),
			"repl edit complete",
			slog.Int("namespace_count", len(m.ast.Namespaces)),
		)

		return m, tea.Println(resultStyle.Render("✔ — AST updated successfully"))

	case editCancelledMsg:
		m.editing = false

		return m, tea.Println(hintStyle.Render("✘ — edit cancelled."))

	case editDeclinedMsg:
		m.editing = false

		return m, tea.Println(hintStyle.Render("edit declined, returning to REPL"))

	case editErrorMsg:
		m.editing = false

		return m, tea.Println(
			errorStyle.Render("✘ — error: " + msg.err.Error()),
		)
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m model) View() string {
	if m.quitting || m.editing {
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
		escKey := suggestionStyle.Render(`Esc`)

		var hint string
		if m.mode == modeEval {
			hint = fmt.Sprintf(
				"Enter an expression to evaluate  •  %s toggle command mode",
				escKey,
			)
		} else {
			hint = strings.Join(ctrlCommands, "  ") +
				fmt.Sprintf("  •  %s return to eval mode", escKey)
		}

		b.WriteString(hintStyle.Render(hint))
		b.WriteString("\n")

	case funcCall.inCall && m.mode == modeEval:
		// When the user has typed a partial word (or is Tab-cycling through
		// candidates), show the completion bar instead of the signature hint.
		// The signature is shown only when the word at the cursor is empty and
		// the user is not actively browsing completions.
		word, _, _ := wordBounds(input, cursor)
		showCandidates := len(m.matches) > 0 && (word != "" || m.tabActive)

		if showCandidates {
			bar := renderCandidateBar(
				m.matches, m.suggIdx, m.tabActive, m.width, true,
			)
			b.WriteString(bar)
			b.WriteString("\n")
		} else {
			signature, params := getSignature(m.ast, funcCall.name)
			if signature != "" {
				hint := renderSignatureHint(signature, params, funcCall.argIndex)
				b.WriteString(hint)
				b.WriteString("\n")
			} else if len(m.matches) > 0 {
				bar := renderCandidateBar(
					m.matches, m.suggIdx, m.tabActive, m.width, true,
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
			m.mode == modeEval,
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
	case tea.KeyCtrlL:
		m, _ = m.switchToMode(modeEval)

		return m, tea.ClearScreen

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
		m.quitting = true

		return m, tea.Quit

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

	case tea.KeyBackspace:
		// Backspace at column 0 in command mode returns to eval mode.
		if m.mode == modeCtrl && m.input.Position() == 0 {
			return m.switchToMode(modeEval)
		}

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

		// ":" as the sole non-whitespace input in eval mode is a shortcut to
		// switch to command mode. The colon itself is discarded — it is not
		// echoed, not added to history, and does not appear as the initial
		// text in command mode.
		if m.mode == modeEval && strings.TrimSpace(m.input.Value()) == ":" {
			m.input.SetValue("")
			m.tabActive = false

			return m.switchToMode(modeCtrl)
		}

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
	// Tab-finish: when not cycling and the typed word at the cursor is
	// already an exact candidate, insert a contextual separator instead
	// of starting to cycle through candidates.
	if !m.tabActive {
		if suffix, ok := m.tryTabFinish(); ok {
			input := m.input.Value()
			cursor := m.input.Position()
			newInput := input[:cursor] + suffix + input[cursor:]
			newCursor := cursor + len(suffix)

			m.input.SetValue(newInput)
			m.input.SetCursor(newCursor)
			m.tabActive = false
			m.suggIdx = -1
			m.matches = nil
			refreshMatches(&m, false)

			return m, nil
		}
	}

	if len(m.matches) == 0 {
		// When input is empty at the top level in eval mode, populate all
		// candidates sorted by priority so the user can browse everything.
		if !populateAllMatches(&m) {
			return m, nil
		}
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
		if !populateAllMatches(&m) {
			return m, nil
		}
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

// populateAllMatches fills m.matches and m.candidates with all top-level
// candidates sorted by priority. It is called when Tab/Shift-Tab is pressed
// with no current matches (i.e. empty input at the top level in eval mode).
// Returns true if matches were populated, false if the operation is a no-op
// (wrong mode or no candidates available).
func populateAllMatches(m *model) bool {
	if m.mode != modeEval {
		return false
	}

	cands := childCandidates(m.ast, "")
	if len(cands) == 0 {
		return false
	}

	all := make(fuzzy.Matches, len(cands))
	for i, c := range cands {
		all[i] = fuzzy.Match{Str: c, Index: i}
	}

	sortMatchesByPriority(all, m.ast)

	m.candidates = cands
	m.matches = all

	// Reset word boundaries to reflect the current word at the cursor so
	// that replaceCurrentWord only replaces the word under the cursor, not
	// the entire input. This is critical when Tab is pressed inside a
	// function argument list (e.g., "filter(keys(|), foo)").
	_, ws, we := wordBounds(m.input.Value(), m.input.Position())
	m.wordStart = ws
	m.wordEnd = we

	// Apply type-based filtering when inside a function call.
	fc := detectFunctionCall(m.input.Value(), m.input.Position())
	if fc.inCall {
		all = filterByParamType(all, m.ast, fc)
		if len(all) == 0 {
			return false
		}

		m.matches = all
	}

	return true
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

	// Apply type-based filtering when inside a function call so that
	// candidates are narrowed to those compatible with the expected
	// parameter type (e.g., only callables for predicate arguments).
	if m.mode == modeEval {
		fc := detectFunctionCall(m.input.Value(), m.input.Position())
		if fc.inCall {
			m.matches = filterByParamType(m.matches, m.ast, fc)
		}
	}

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

	formatted := lang.FormatResult(result)

	var printCmd tea.Cmd
	if _, ok := result.(*lang.FuncRef); ok {
		// Render function references with hint styling to distinguish them from
		// evaluated values.
		printCmd = tea.Println(hintStyle.Render(formatted))
	} else {
		printCmd = tea.Println(resultStyle.Render(formatted))
	}

	return m, tea.Sequence(echoCmd, printCmd)
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
		m, _ = m.switchToMode(modeEval)

		var helpText string
		if len(args) > 0 {
			helpText = m.helpView(args[0])
		} else {
			helpText = m.helpView("")
		}

		return m, tea.Sequence(echoCmd, tea.Println(helpText))

	case "l", "list":
		m, _ = m.switchToMode(modeEval)

		return m, tea.Sequence(echoCmd, tea.Println(m.listNamespaces()))

	case "c", "clear":
		m, _ = m.switchToMode(modeEval)

		return m, tea.Sequence(echoCmd, tea.ClearScreen)

	case "e", "edit":
		m, _ = m.switchToMode(modeEval)
		m.editing = true

		var editCmd tea.Cmd

		m, editCmd = m.handleEdit(m.ctxFunc(), formatCtrlCommand(input))

		return m, editCmd

	default:
		// Unknown command — stay in command mode so the user can retry.
		return m, tea.Sequence(
			echoCmd,
			tea.Println(errorStyle.Render("Unknown command: "+cmd+" (try 'help')")),
		)
	}
}

func (m model) handleEdit(_ context.Context, echo string) (model, tea.Cmd) {
	cmd := &editASTCommand{
		ast:     m.ast,
		ctxFunc: m.ctxFunc,
		logger:  m.logger,
		echo:    echo,
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

		return editASTMsg{ast: cmd.newAST, unchanged: cmd.unchanged}
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

// helpView returns the help text. When topic is empty, the overview is
// returned; otherwise the detailed text for the given topic is returned.
func (m model) helpView(topic string) string {
	if topic == "" {
		return helpOverview(m.pretty)
	}

	return globalHelpTopics.section(topic, m.pretty)
}

func (m model) listNamespaces() string {
	var b strings.Builder

	// Collect namespaces and sort alphabetically.
	type entry struct {
		name    string
		preview string
	}

	entries := make([]entry, 0, len(m.ast.Namespaces))

	for ns := range m.ast.All() {
		entries = append(entries, entry{
			name:    ns.Name,
			preview: formatPreview(ns),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// Find max name length for alignment.
	maxLen := 0
	for _, e := range entries {
		if len(e.name) > maxLen {
			maxLen = len(e.name)
		}
	}

	for _, e := range entries {
		pad := strings.Repeat(" ", maxLen-len(e.name)+2)
		fmt.Fprintf(&b, "  %s%s%s\n", e.name, pad, hintStyle.Render(e.preview))
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
