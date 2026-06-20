//go:build e2e

package cli

import (
	"bytes"
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

var ansiCSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string {
	return ansiCSI.ReplaceAllString(s, "")
}

func TestRepl_E2E_MultilineEval_NoHorizontalDrift(t *testing.T) {
	out := &e2eOutput{}
	p := tea.NewProgram(
		newLoopWithHistory(loadHistory("")),
		tea.WithOutput(out),
		tea.WithoutSignals(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	p.Send(tea.WindowSizeMsg{Width: 40, Height: 8})
	p.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p.Send(tea.KeyPressMsg{Code: 'w', Text: "w"})
	p.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p.Send(tea.KeyPressMsg{Code: 'w', Text: "w"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "\n"})
	p.Send(tea.KeyPressMsg{Code: 'w', Text: "w"})
	p.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})

	teatest.WaitFor(
		t,
		out,
		func(b []byte) bool {
			s := stripANSI(string(b))
			return strings.Contains(s, "ewew") && strings.Contains(s, "\nwe")
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
	if regexp.MustCompile(`\S\s{30,}\S`).MatchString(snapshot) {
		t.Fatalf("detected horizontal spacing drift in transcript output:\n%s", snapshot)
	}
}

func TestRepl_E2E_MultilineEval_NoExcessBlankLines(t *testing.T) {
	out := &e2eOutput{}
	p := tea.NewProgram(
		newLoopWithHistory(loadHistory("")),
		tea.WithOutput(out),
		tea.WithoutSignals(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	p.Send(tea.WindowSizeMsg{Width: 40, Height: 4})
	p.Send(tea.KeyPressMsg{Code: 'a', Text: "a"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "\n"})
	p.Send(tea.KeyPressMsg{Code: 'b', Text: "b"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "\n"})
	p.Send(tea.KeyPressMsg{Code: 'c', Text: "c"})
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})

	teatest.WaitFor(
		t,
		out,
		func(b []byte) bool {
			s := stripANSI(string(b))
			return strings.Contains(s, "a") && strings.Contains(s, "b") && strings.Contains(s, "c")
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
	if strings.Contains(snapshot, "\n\n\n") {
		t.Fatalf("detected excess blank lines in transcript output:\n%s", snapshot)
	}
}
