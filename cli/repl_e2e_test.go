//go:build e2e

package cli

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest"
)

type e2eOutput struct {
	mu   sync.Mutex
	live bytes.Buffer
	full bytes.Buffer
}

func (o *e2eOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, _ = o.full.Write(p)
	return o.live.Write(p)
}

func (o *e2eOutput) Read(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.live.Read(p)
}

func (o *e2eOutput) Snapshot() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.full.String()
}

var (
	// CSI sequences (ESC '[' ... final byte), including private-mode
	// parameter prefixes such as '<', '=', '>' (e.g. Kitty keyboard protocol
	// enable/disable, modifyOtherKeys).
	ansiCSI = regexp.MustCompile(`\x1b\[[0-9;:<=>?]*[ -/]*[@-~]`)
	// OSC sequences (ESC ']' ... terminated by BEL or ST), e.g. the
	// background-color query bubbletea sends on startup.
	ansiOSC = regexp.MustCompile(`\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
)

func stripANSI(s string) string {
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiCSI.ReplaceAllString(s, "")
	// Raw-mode terminal output uses "\r\n" line endings; normalize to "\n"
	// so plain string/line comparisons in tests don't need to account for
	// the carriage return.
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// waitForFocus gives the program time to process the async ready cmd
// triggered by tea.WindowSizeMsg (see repl.go's WindowSizeMsg case), which
// focuses the editor. Sending keys before this round trip completes would
// have them silently dropped by the unfocused textinput/textarea.
func waitForFocus() {
	time.Sleep(50 * time.Millisecond)
}

func TestRepl_E2E_MultilineEval_EmitsSerializedResult(t *testing.T) {
	out := &e2eOutput{}
	p := tea.NewProgram(
		makeREPL(context.Background(), withHistory("")),
		tea.WithOutput(out),
		tea.WithoutSignals(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	p.Send(tea.WindowSizeMsg{Width: 40, Height: 8})
	// WindowSizeMsg triggers an async ready cmd round trip that focuses the
	// editor; give it time to land before sending keys, otherwise the
	// unfocused textinput/textarea silently drops them.
	waitForFocus()
	p.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p.Send(tea.KeyPressMsg{Code: 'w', Text: "w"})
	p.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p.Send(tea.KeyPressMsg{Code: 'w', Text: "w"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.Send(tea.KeyPressMsg{Code: 'w', Text: "w"})
	p.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})

	teatest.WaitFor(
		t,
		out,
		func(b []byte) bool {
			s := stripANSI(string(b))
			return strings.Contains(s, `"src":"ewew\nwe"`)
		},
		teatest.WithDuration(3*time.Second),
		teatest.WithCheckInterval(10*time.Millisecond),
	)

	p.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit in time")
	}

	snapshot := stripANSI(out.Snapshot())
	if !strings.Contains(snapshot, `"src":"ewew\nwe"`) {
		t.Fatalf("expected serialized multiline result in transcript output, got:\n%s", snapshot)
	}
}

func TestRepl_E2E_MultilineEval_EmitsContiguousSerializedTranscript(t *testing.T) {
	out := &e2eOutput{}
	p := tea.NewProgram(
		makeREPL(context.Background(), withHistory("")),
		tea.WithOutput(out),
		tea.WithoutSignals(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	p.Send(tea.WindowSizeMsg{Width: 40, Height: 4})
	waitForFocus()
	p.Send(tea.KeyPressMsg{Code: 'a', Text: "a"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.Send(tea.KeyPressMsg{Code: 'b', Text: "b"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.Send(tea.KeyPressMsg{Code: 'c', Text: "c"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})

	teatest.WaitFor(
		t,
		out,
		func(b []byte) bool {
			s := stripANSI(string(b))
			return strings.Contains(s, `"src":"a\nb\nc"`)
		},
		teatest.WithDuration(3*time.Second),
		teatest.WithCheckInterval(10*time.Millisecond),
	)

	p.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit in time")
	}

	// The raw accumulated capture spans many redraw frames. Non-altscreen
	snapshot := stripANSI(out.Snapshot())
	if !strings.Contains(snapshot, `"src":"a\nb\nc"`) {
		t.Fatalf("expected serialized multiline result in transcript output, got:\n%s", snapshot)
	}
}
