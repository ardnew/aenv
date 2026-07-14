package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
	"github.com/ardnew/aenv/pkg"
)

const historyFile = "history"

const maxOutputLines = 2048

// logQueueSize bounds the number of in-flight log lines waiting to be
// delivered to the running [tea.Program]. Write (see logsink.go) blocks the
// calling goroutine once this buffer is full (backpressure) rather than
// spawning an unbounded goroutine per log line.
const logQueueSize = 256

// repl is the REPL's [tea.Model].
//
// Its behavior is implemented across several files, grouped by concern:
//   - repl.go: model definition, lifecycle (Init/Update dispatch), construction.
//   - keyrouter.go: key bindings and key-press routing/actions.
//   - pipeline.go: the collect/capture/commit/evaluate/reset eval cycle.
//   - output.go: the output buffer/viewport and View rendering.
//   - logsink.go: wiring the REPL as the destination for terminal log output.
type repl struct {
	app *tea.Program
	ctx context.Context

	edit TextEdit

	keys keyMap
	hist history

	ast lang.AST

	screen     viewport.Model
	altScreen  bool
	altHeight  int
	buffer     []string
	bufferText string // cached strings.Join(buffer, "\n"), see appendOutput

	quitting bool

	logQ chan []byte
	log1 *sync.Once
}

// msgAttr groups per-message-type structured log fields under a key derived
// from the message's Go type, keeping log entries traceable to the exact
// Update case that produced them.
func msgAttr(msg tea.Msg, kv ...any) []slog.Attr {
	return log.Group(fmt.Sprintf("%T", msg), kv...)
}

// makeREPL creates a new [repl] with default settings, except for those
// overridden by any provided [option].
func makeREPL(ctx context.Context, opts ...option[repl]) repl {
	v := viewport.New()
	// Keep viewport scroll behavior on explicit navigation keys, but avoid
	// intercepting plain text input that collides with default bindings.
	v.KeyMap.PageDown = key.NewBinding(key.WithKeys("pgdown"))
	v.KeyMap.PageUp = key.NewBinding(key.WithKeys("pgup"))
	v.KeyMap.HalfPageDown.Unbind()
	v.KeyMap.HalfPageUp.Unbind()

	// Initialize with defaults then apply opts to override.
	r := repl{
		edit:   makeTextEdit(),
		keys:   defaultKeyMap(),
		hist:   loadHistory(pkg.CachePath(historyFile)),
		screen: v,
		logQ:   make(chan []byte, logQueueSize),
		log1:   new(sync.Once),
	}
	return wrap(r, append(opts, withProgram(ctx))...)
}

func withProgram(ctx context.Context) option[repl] {
	return func(l *repl) {
		l.ctx = ctx
		l.app = tea.NewProgram(l, tea.WithContext(ctx))
	}
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

// Update is the REPL's central message dispatcher. It is intentionally a thin
// table mapping each message type to the method that handles it -- read it
// top-to-bottom as a table of contents; the substantive logic for each
// concern lives in the file named alongside its case below.
//
// A few cases are trivial enough (2-4 lines, no shared state with other
// cases) that extracting a named method would add indirection without
// clarity benefit, so they remain inline: tea.WindowSizeMsg,
// tea.BackgroundColorMsg, faultMsg and quitMsg.
func (l repl) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.altHeight = msg.Height - 1
		l.edit = l.edit.setSize(tea.Position{X: msg.Width, Y: msg.Height})
		l = l.syncViewportSize()
		return l, ready

	case tea.BackgroundColorMsg:
		log.Trace(msgAttr(msg, "color", msg.Color, "isDark", msg.IsDark()))
		l.edit = l.edit.setStyle(msg.IsDark())
		return l, nil

	case tea.MouseWheelMsg: // output.go
		return l.handleMouseWheel(msg)

	case readyMsg: // logsink.go
		return l.handleReady()

	case logMsg: // output.go
		return l.handleLog(msg)

	case faultMsg:
		log.Error(msgAttr(msg, fmt.Sprintf("%T", msg.err), msg.err.Error()))
		return l, quit

	case setEditModeMsg: // keyrouter.go
		return l.handleSetEditMode(msg)

	case collectMsg: // pipeline.go
		return l.handleCollect(msg)

	case captureMsg: // pipeline.go
		return l.handleCapture(msg)

	case commitMsg: // pipeline.go
		return l.handleCommit(msg)

	case evaluateMsg: // pipeline.go
		return l.handleEvaluate(msg)

	case resetMsg: // pipeline.go
		return l.handleReset(msg)

	case quitMsg:
		log.Trace(msgAttr(msg))
		return l, tea.Quit

	case tea.KeyPressMsg: // keyrouter.go
		return l.handleKeyPress(msg)
	}
	return l, nil
}

// View renders the REPL. See output.go for transcriptView/altScreenView,
// which hold the two rendering modes' logic.
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
	//      :┃ This is a previously evaluated prompt       :←Prev REPL input
	//      :┃ containing multiple lines. All input        :
	//   ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
	//   ┃  :┃ lines are prefixed WITHOUT line numbers,    :  ┃
	//   ┃  :┃ for simpler copy-pasting.                   :  ┃
	//   ┃  :┃                                             :  ┃
	//   ┃  :┃ Only internal newlines are kept.            :  ┃
	//   ┃  :··············································:  ┃
	//   ┃  : This is a previously evaluated prompt        :←Prev REPL output
	//   ┃  : containing multiple lines. All input         :  ┃
	//   ┃  : lines are prefixed WITHOUT line numbers,     :  ┃
	//   ┃  : for simpler copy-pasting.                    :  ┃
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
	var cursor *tea.Cursor
	l.edit.cursor(
		func(virtual bool, c *tea.Cursor) {
			if !virtual {
				cursor = c
			}
		},
	)

	if l.altScreen {
		return l.altScreenView(cursor)
	}
	return l.transcriptView(cursor)
}

func repLoop(ctx context.Context, ast lang.AST) error {
	log.Debug(log.Attrs("history", pkg.CachePath(historyFile)))
	l := makeREPL(
		ctx,
		withKeyMap(defaultKeyMap()),
		withHistory(pkg.CachePath(historyFile)),
		withAST(ast),
	)

	_, err := l.app.Run()
	return err
}
