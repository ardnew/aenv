package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"
)

const historyFile = "history"

// logMsg is used to queue a message written to the REPL output stream.
//
// This is required for synchronizing I/O with the REPL whether alt-screen mode
// is enabled or not.
type logMsg struct {
	template string
	args     []any
}

// makeLogMsg is a helper for constructing logMsg with optional printf-style
// formatting.
//
// It replaces noisy struct and slice literals at call sites with concise
// function call syntax.
func makeLogMsg(template string, args ...any) logMsg {
	return logMsg{template, args}
}

type faultMsg struct{ err error }

func fault(err error) tea.Cmd {
	return func() tea.Msg { return faultMsg{err} }
}

// Executing REPL input requires 4 render cycles to synchronize the persistent
// models and output stream without visual artifacts.
type (
	collectMsg  struct{ input string }      // 1. clear in-focus styling
	captureMsg  struct{ input string }      // 2. capture unfocused input; reset
	commitMsg   struct{ text, view string } // 3. draw captured input
	evaluateMsg struct{ input string }      // 4. evaluate input; draw output
)

func collect(input string) tea.Cmd     { return func() tea.Msg { return collectMsg{input} } }
func capture(input string) tea.Cmd     { return func() tea.Msg { return captureMsg{input} } }
func commit(text, view string) tea.Cmd { return func() tea.Msg { return commitMsg{text, view} } }
func evaluate(input string) tea.Cmd    { return func() tea.Msg { return evaluateMsg{input} } }

type (
	readyMsg struct{}
	resetMsg struct{}
	quitMsg  struct{}
)

func ready() tea.Msg { return readyMsg{} }
func reset() tea.Msg { return resetMsg{} }
func quit() tea.Msg  { return quitMsg{} }

func inputLineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func tailLines(text string, n int) string {
	if n <= 0 || text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= n {
		return text
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

type repl struct {
	app *tea.Program

	edit TextEdit

	keys keyMap
	hist history

	ast lang.AST

	altScreen bool
	altHeight int
	output    []string
	ready     bool
	quitting  bool
}

const maxOutputLines = 2048

func (l repl) appendOutput(text string) repl {
	trimmed := strings.TrimRight(text, "\r\n")
	if trimmed == "" {
		l.output = append(l.output, "")
	} else {
		l.output = append(l.output, strings.Split(trimmed, "\n")...)
	}
	if drop := len(l.output) - maxOutputLines; drop > 0 {
		l.output = append([]string(nil), l.output[drop:]...)
	}
	return l
}

type keyMap struct {
	eval key.Binding
	exec key.Binding
	quit key.Binding

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
			eval: key.NewBinding(
				key.WithKeys("alt+enter"),
				key.WithHelp("alt+enter", "eval"),
			),
			exec: key.NewBinding(
				key.WithKeys("ctrl+d"),
				key.WithHelp("ctrl+d", "eval (EOF)"),
			),
			quit: key.NewBinding(
				key.WithKeys("ctrl+c", "ctrl+q"),
				key.WithHelp("ctrl+c", "quit"),
			),
			source: key.NewBinding(
				key.WithKeys("ctrl+o"),
				key.WithHelp("ctrl+o", "source"),
			),
			format: key.NewBinding(
				key.WithKeys("ctrl+f"),
				key.WithHelp("ctrl+f", "format"),
			),
			preview: key.NewBinding(
				key.WithKeys("ctrl+p"),
				key.WithHelp("ctrl+p", "preview"),
			),
			screen: key.NewBinding(
				key.WithKeys("alt+s"),
				key.WithHelp("alt+s", "screen"),
			),
			toggle: key.NewBinding(
				key.WithKeys("alt+e"),
				key.WithHelp("alt+e", "toggle input mode"),
			),
			prev: key.NewBinding(
				key.WithKeys("up"),
			),
			next: key.NewBinding(
				key.WithKeys("down"),
			),
		}
	},
)

// makeREPL creates a new [repl] with default settings, except for those
// overridden by any provided [option].
func makeREPL(ctx context.Context, opts ...option[repl]) repl {
	e := makeTextEdit()

	// Initialize with defaults then apply opts to override.
	r := repl{
		edit: e,
		keys: defaultKeyMap(),
		hist: loadHistory(pkg.CachePath(historyFile)),
	}
	return wrap(r, append(opts, withProgram(ctx))...)
}

func withProgram(ctx context.Context) option[repl] {
	return func(l *repl) { l.app = tea.NewProgram(l, tea.WithContext(ctx)) }
}

func withKeyMap(keys keyMap) option[repl] {
	return func(l *repl) { l.keys = keys }
}

func withHistory(path string) option[repl] {
	return func(l *repl) { l.hist = loadHistory(path) }
}

func withAST(ast lang.AST) option[repl] {
	return func(l *repl) { l.ast = ast }
}

func (l repl) Init() tea.Cmd {
	return tea.Batch(l.edit.Init(), tea.RequestBackgroundColor)
}

func (l repl) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Accumulate (instead of wrap) to execute concurrent cmds in the same update.
	// TODO: Would be nice to verify BubbleTea/Ultraviolet actually supports this.
	var batch []tea.Cmd
	var forwardText bool
	var err error

	// Enclose all message attrs in a group keyed by the message type.
	msgAttr := func(kv ...any) []slog.Attr {
		return log.Group(fmt.Sprintf("%T", msg), kv...)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.altHeight = msg.Height - 1
		l.ready = true
		l.edit = l.edit.setSize(makeMaxBounds(msg.Width, msg.Height))
		log.Tracef(
			log.Attrs("size", l.edit.size,
				slog.GroupAttrs(l.edit.mode.String(),
					log.Attrs("dim", l.edit.activeDim())...,
				),
			),
			"resize edit",
		)
		return l, ready

	case tea.BackgroundColorMsg:
		log.Trace(msgAttr("color", msg.Color, "isDark", msg.IsDark()))
		l.edit = l.edit.setBackgroundColor(msg.Color, msg.IsDark())

	case readyMsg:
		if l, err = l.onReady(); err != nil {
			return l, fault(err)
		}
		l.ready = true

	case logMsg:
		if !l.ready {
			return l, nil
		}
		s := fmt.Sprintf(msg.template, msg.args...)
		if l.altScreen {
			l = l.appendOutput(s)
			return l, nil
		}
		return l, tea.Println(strings.TrimRight(s, "\r\n"))

	case faultMsg:
		log.Error(msgAttr(fmt.Sprintf("%T", msg.err), msg.err.Error()))
		return l, quit

	case collectMsg:
		// Normalize the input, save to history and forward to next command.
		text := strings.TrimRight(msg.input, "\r\n")
		log.Trace(msgAttr(
			"mode", l.edit.mode,
			"bytes", len(text),
			"lines", inputLineCount(text),
		))
		// Write unstyled text to history so that it can be recalled directly
		// without formatting artifacts.
		l.hist.record(text)
		// Remove focus and capture the view in the next render cycle.
		l.edit = l.edit.setValue("").setFocus(editNone)
		return l, capture(text)

	case captureMsg:
		log.Trace(msgAttr("mode", l.edit.mode))
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
			view = makeLineEdit(msg.input).View()
		case editArea:
			view = makeAreaEdit(msg.input).View()
		}
		return l, tea.Sequence(reset, commit(msg.input, view.Content))

	case commitMsg:
		log.Trace(msgAttr("mode", l.edit.mode))
		if l.altScreen {
			l = l.appendOutput(msg.view)
			return l, evaluate(msg.text)
		}
		return l, tea.Sequence(tea.Println(msg.view), evaluate(msg.text))

	case evaluateMsg:
		log.Debug(msgAttr("mode", l.edit.mode))
		// evaluate is defined with a value receiver for immutability.
		r, output, err := l.evaluate(msg.input)
		if err != nil {
			// Return the original [repl] to avoid preserving an invalid or incomplete
			// AST in its model, which could otherwise reproduce related errors.
			return l, fault(err)
		}
		if l.altScreen {
			r = r.appendOutput(output)
		} else {
			batch = append(batch, tea.Println(output))
		}
		if l.quitting {
			batch = append(batch, quit)
		}
		return r, tea.Sequence(batch...)

	case resetMsg:
		log.Trace(msgAttr("mode", l.edit.mode))
		l.edit = l.edit.reset()
		l.edit = l.edit.setFocus(l.edit.mode)

	case quitMsg:
		log.Trace(msgAttr())
		return l, tea.Quit

	case tea.KeyPressMsg:
		forwardText = true

		log.Trace(msgAttr("code", msg.Code, "text", msg.Text, "mod", msg.Mod))

		switch {
		case key.Matches(msg, l.keys.eval):
			log.Debug(msgAttr("action", "eval"))
			return l, collect(l.edit.currentValue())

		case key.Matches(msg, l.keys.exec):
			log.Debug(msgAttr("action", "exec"))
			l.quitting = true
			return l, collect(l.edit.currentValue())

		case key.Matches(msg, l.keys.quit):
			log.Debug(msgAttr("action", "quit"))
			return l, tea.Quit

		case key.Matches(msg, l.keys.source):
			log.Debug(msgAttr("action", "source"))

		case key.Matches(msg, l.keys.format):
			log.Debug(msgAttr("action", "format"))

		case key.Matches(msg, l.keys.preview):
			log.Debug(msgAttr("action", "preview"))

		case key.Matches(msg, l.keys.screen):
			log.Debug(msgAttr("action", "toggle", "alt-screen", !l.altScreen))
			l.altScreen = !l.altScreen
			// l.edit = wrap(l.edit, withAltScreen(l.altScreen, l.altHeight))
			// maxHeight := l.edit.size.y.max
			// l.edit.SetMaxHeight(l.altHeight)
			// l.altHeight = maxHeight

		case key.Matches(msg, l.keys.toggle) || (msg.Code == 'e' && msg.Mod == tea.ModAlt):
			log.Debug(msgAttr("action", "toggle", "edit-mode", l.edit.mode))
			value := l.edit.currentValue()
			l.edit = l.edit.setValue(value)
			l.edit.mode = nextEditMode(l.edit.mode)
			l.edit = l.edit.setFocus(l.edit.mode).moveCursorEnd()

		case msg.Code == tea.KeyEnter && msg.Mod == 0 && l.edit.isLineMode():
			log.Debug(log.Attrs("action", "submit"), "REPL")
			return l, collect(l.edit.currentValue())

		case key.Matches(msg, l.keys.prev):
			if l.edit.atFirstLine() {
				if value, ok := l.hist.prev(l.edit.currentValue()); ok {
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
	}

	if forwardText {
		edit, cmd := l.edit.Update(msg)
		if text, ok := edit.(TextEdit); ok {
			l.edit = text
		}
		batch = append(batch, cmd)
	}
	return l, tea.Batch(batch...)
}

func (l repl) View() tea.View {
	// The non-alt-screen REPL writes to the terminal buffer directly as a
	// transcript showing all previous input prompts followed by their output.
	//
	// This is conventional REPL behavior and allows users to use their terminal
	// scrollback to review all previous interactions.
	//
	// The following illustrates an example transcript in a terminal with a
	// scrollback buffer. The components are labeled to define the terminology
	// used in the rest of the code and comments.
	//
	// The dotted lines are logical dividers separating sections of the
	// transcript. They are for illustration purposes only and are not actually
	// rendered.
	//
	// The outer solid lines represent the terminal window. The transcript
	// extends beyond the bounds of the terminal window to represent the
	// scrollback buffer.
	//
	//      ↓ Transcript in scrollback buffer
	//      ················································
	//      : $ aenv eval                                  :←Shell prompt
	//      :··············································:
	//      :┃ 1  This is a previously evaluated prompt    :←Prev REPL input
	//      :┃ 2  containing multiple lines. All input     :
	//   ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	//   ┃  :┃ 3  lines are prefixed with line numbers.    :  ┃
	//   ┃  :┃ 4                                           :  ┃
	//   ┃  :┃ 5  Only internal newlines are kept.         :  ┃
	//   ┃  :··············································:  ┃
	//   ┃  : This is a previously evaluated prompt        :←Prev REPL output
	//   ┃  : containing multiple lines. All input         :  ┃
	//   ┃  : lines are prefixed with line numbers.        :  ┃
	//   ┃  :                                              :  ┃
	//   ┃  : Only internal newlines are kept.             :  ┃
	//   ┃  :··············································:  ┃
	//   ┃  :┃ 1  This is the current prompt that has not  :←Current REPL input
	//   ┃  :┃ 2  yet been evaluated.                      :  ┃
	//   ┃  :┃ 3 ░░░█░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ :←Active cursor (col=3)
	//   ┃  :┃ 4  Line 3 is the only highlighted line in   :  ┃
	//   ┃  :┃ 5  the entire transcript because it         :  ┃
	//   ┃  :┃ 6  contains the cursor.                     :  ┃
	//   ┃  ················································  ┃
	//   ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
	//   ↑ Physical terminal window with scrollback buffer
	var v tea.View

	var c *tea.Cursor
	l.edit.cursor(
		func(virtual bool, cursor *tea.Cursor) {
			if !virtual {
				c = cursor
			}
		},
	)

	v.SetContent(l.edit.View().Content)
	cursor := c
	if l.altScreen {
		editContent := l.edit.View().Content
		editLines := max(1, inputLineCount(editContent))
		keep := max(0, l.edit.size.y.max-editLines)
		output := ""
		if len(l.output) > 0 {
			output = tailLines(strings.Join(l.output, "\n"), keep)
		}
		if output == "" {
			v.SetContent(editContent)
		} else {
			v.SetContent(output + "\n" + editContent)
		}
		if c != nil {
			shifted := *c
			shifted.Y += inputLineCount(output)
			cursor = &shifted
		}
	}
	v.Cursor = cursor
	v.AltScreen = l.altScreen

	return v
}

// IsTerminalWriter implements the [log.TerminalWriter] interface, which allows
// the [repl] to be used as a log handler output.
func (repl) IsTerminalWriter() bool { return true }

// Write implements the [io.Writer] interface, which allows the [repl] to be
// used as a log handler output.
func (l repl) Write(p []byte) (n int, err error) {
	go l.app.Send(makeLogMsg("%s", p))
	return len(p), nil
}

func (l repl) evaluate(input string) (repl, string, error) {
	attrs := log.Attrs(
		"bytes", len(input),
		"lines", inputLineCount(input),
	)
	log.Trace(attrs, "parsing input")

	// TODO: replace with actual evaluation
	output := input

	_, err := l.ast.ReadFrom(strings.NewReader(input))
	if err != nil {
		attrs = append(attrs, log.Attrs("error", err)...)
		if pos := l.ast.Pos(); !pos.IsZero() {
			attrs = append(attrs, log.Attrs("position", pos.String())...)
		}
		log.Debug(attrs)
		return l, "",
			lang.MakeParseError(err, l.ast.Pos(), strings.NewReader(input))
	}
	log.Debug(attrs, "evaluate")

	return l, output, nil
}

func (l repl) onReady() (repl, error) {
	return l, log.MapHandlers(log.IsTerminalHandler,
		func(h *log.Handler) error { return h.SetWriter(l) },
	)
}

func repLoop(ctx context.Context, ast lang.AST) error {
	log.Debug(log.Attrs("history", pkg.CachePath(historyFile)), "initialize REPL")
	l := makeREPL(
		ctx,
		withKeyMap(defaultKeyMap()),
		withHistory(pkg.CachePath(historyFile)),
		withAST(ast),
	)

	_, err := l.app.Run()
	return err
}
