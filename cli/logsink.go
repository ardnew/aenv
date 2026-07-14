package cli

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/ardnew/aenv/log"
)

// IsTerminalWriter implements the [log.TerminalWriter] interface, which allows
// the [repl] to be used as a log handler output.
func (repl) IsTerminalWriter() bool { return true }

// Write implements the [io.Writer] interface, which allows the [repl] to be
// used as a log handler output.
//
// Log lines are queued onto logq and relayed to the running [tea.Program], in
// order, by a single background goroutine (drainLog, started once by
// handleReady) instead of spawning a new goroutine per call. Write blocks the
// calling goroutine once the queue is full (bounded backpressure) or l.ctx is
// done, whichever comes first.
func (l repl) Write(p []byte) (n int, err error) {
	b := slices.Clone(p)
	select {
	case l.logQ <- b:
	case <-l.ctx.Done():
	}
	return len(p), nil
}

// onReady registers the [repl] as the output destination for terminal log
// handlers, so log output is routed through the REPL's own message loop
// instead of writing directly to the terminal (which would corrupt the TUI).
func (l repl) onReady() (repl, error) {
	return l, log.MapHandlers(log.IsTerminalHandler,
		func(h *log.Handler) error { return h.SetWriter(l) },
	)
}

// handleReady focuses the active editor, wires up the REPL as the terminal
// log destination (onReady), and starts the drainLog goroutine exactly once
// (guarded by logOnce -- readyMsg can fire more than once, e.g. on every
// terminal resize).
//
// NOTE: this case fell through to Update's shared tail (syncViewportSize)
// rather than returning early; that is reproduced explicitly here.
func (l repl) handleReady() (repl, tea.Cmd) {
	var focus tea.Cmd
	l.edit, focus = l.edit.setFocus(l.edit.mode)

	r, err := l.onReady()
	if err != nil {
		return r, fault(err)
	}
	l = r
	l.log1.Do(func() { go l.drainLog() })

	return l.syncViewportSize(), focus
}

// drainLog relays log lines queued by Write to the running [tea.Program] one
// at a time, preserving arrival order. It exits once l.ctx is done (real
// program shutdown, or automatic test-context cancellation), so no manual
// cleanup is required by callers or tests.
func (l repl) drainLog() {
	for {
		select {
		case <-l.ctx.Done():
			return
		case b, ok := <-l.logQ:
			if !ok {
				return
			}
			l.app.Send(makeLogMsg("%s", b))
		}
	}
}
