package repl

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// makeMatches builds a fuzzy.Matches slice from a plain string slice with no
// match highlights — sufficient for testing the candidate bar layout.
func makeMatches(names []string) fuzzy.Matches {
	m := make(fuzzy.Matches, len(names))
	for i, n := range names {
		m[i] = fuzzy.Match{Str: n, Index: i}
	}

	return m
}

// TestRenderCandidateBar_NoScroll verifies that when all candidates fit inside
// the terminal width no scroll arrows are emitted.
func TestRenderCandidateBar_NoScroll(t *testing.T) {
	matches := makeMatches([]string{"alpha", "beta", "gamma"})
	result := renderCandidateBar(matches, -1, false, 200)

	if strings.Contains(result, "←") || strings.Contains(result, "→") {
		t.Errorf("expected no scroll arrows, got: %q", result)
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(result, name) {
			t.Errorf("expected %q in result, got: %q", name, result)
		}
	}
}

// TestRenderCandidateBar_RightArrow verifies that a right-scroll arrow is
// shown when the candidate list overflows to the right with no selection.
func TestRenderCandidateBar_RightArrow(t *testing.T) {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	matches := makeMatches(names)

	// Use a very narrow width so only a couple of candidates fit.
	narrowWidth := lipgloss.Width("alpha  beta") + 2
	result := renderCandidateBar(matches, -1, false, narrowWidth)

	if !strings.Contains(result, "→") {
		t.Errorf("expected right arrow, got: %q", result)
	}

	if strings.Contains(result, "←") {
		t.Errorf("expected no left arrow at start of list, got: %q", result)
	}
}

// TestRenderCandidateBar_SelectedAlwaysVisible verifies that when tab-cycling
// the selected candidate is always present in the rendered output even when
// the list is wider than the terminal.
func TestRenderCandidateBar_SelectedAlwaysVisible(t *testing.T) {
	names := []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", "hhh"}
	matches := makeMatches(names)

	// Width that fits roughly 3 short names.
	w := lipgloss.Width("aaa  bbb  ccc  ←   →") + 4

	for suggIdx := range names {
		result := renderCandidateBar(matches, suggIdx, true, w)

		if !strings.Contains(result, names[suggIdx]) {
			t.Errorf("suggIdx=%d: selected candidate %q not visible in %q",
				suggIdx, names[suggIdx], result)
		}
	}
}

// TestRenderCandidateBar_LeftArrowWhenScrolled verifies that a left-scroll
// arrow is shown when the window has moved past the first candidate.
func TestRenderCandidateBar_LeftArrowWhenScrolled(t *testing.T) {
	names := []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", "hhh"}
	matches := makeMatches(names)

	// Width that fits roughly 3 short names.
	w := lipgloss.Width("aaa  bbb  ccc  ←   →") + 4

	// Selecting a later candidate should force the window to scroll,
	// revealing the left arrow.
	result := renderCandidateBar(matches, len(names)-1, true, w)

	if !strings.Contains(result, "←") {
		t.Errorf("expected left arrow when scrolled, got: %q", result)
	}

	if !strings.Contains(result, names[len(names)-1]) {
		t.Errorf("expected last candidate to be visible, got: %q", result)
	}
}

func TestWordBounds_ExprOperators(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		cursor    int
		wantWord  string
		wantStart int
		wantEnd   int
	}{
		{"simple", "foo", 3, "foo", 0, 3},
		{"dot_separated", "bar.baz", 7, "baz", 4, 7},
		{"after_plus", "a + fo", 6, "fo", 4, 6},
		{"after_paren", "double(fo", 9, "fo", 7, 9},
		{"after_comma", "add(a, fo", 9, "fo", 7, 9},
		{"in_ternary", "x ? fo", 6, "fo", 4, 6},
		{"after_comparison", "a > fo", 6, "fo", 4, 6},
		{"empty_at_boundary", "a + ", 4, "", 4, 4},
		{"mid_word", "foobar", 3, "foobar", 0, 6},
		{"at_start", "foo", 0, "foo", 0, 3},
		{"between_operators", "a+b", 2, "b", 2, 3},
		// Hyphens are part of identifiers, not word boundaries.
		{"hyphenated", "log-pretty", 10, "log-pretty", 0, 10},
		{"hyphenated_after_dot", "config.log-pretty", 17, "log-pretty", 7, 17},
		{"hyphenated_partial", "config.log-pr", 13, "log-pr", 7, 13},
		// After dot is an empty word (for triggering child completions).
		{"empty_after_dot", "config.", 7, "", 7, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			word, start, end := wordBounds(tt.input, tt.cursor)
			if word != tt.wantWord || start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("wordBounds(%q, %d) = (%q, %d, %d), want (%q, %d, %d)",
					tt.input, tt.cursor, word, start, end,
					tt.wantWord, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestParentPath_WithOperators(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wordStart int
		want      string
	}{
		{"top_level", "fo", 0, ""},
		{"simple_chain", "bar.baz.", 8, "bar.baz"},
		{"after_operator", "foo + bar.baz.", 14, "bar.baz"},
		{"after_paren", "(bar.baz.", 9, "bar.baz"},
		{"no_chain", "a + ", 4, ""},
		{"deep_chain", "a.b.c.", 6, "a.b.c"},
		{"after_equals", "x = a.b.", 8, "a.b"},
		// Hyphens are part of identifiers in the parent path.
		{"hyphenated_chain", "config.log-pretty.", 18, "config.log-pretty"},
		{"hyphenated_after_op", "x + config.log-pretty.", 22, "config.log-pretty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parentPath(tt.input, tt.wordStart)
			if got != tt.want {
				t.Errorf("parentPath(%q, %d) = %q, want %q",
					tt.input, tt.wordStart, got, tt.want)
			}
		})
	}
}
