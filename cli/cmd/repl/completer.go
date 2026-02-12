package repl

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/ardnew/aenv/lang"
)

// ctrlCommands are the available control-mode commands.
var ctrlCommands = []string{"help", "list", "edit", "clear", "quit"}

// isWordBoundary returns true if the rune is a word delimiter for completion
// purposes. This includes whitespace, the member-access dot, and expr-lang
// operator/punctuation characters. Hyphens are intentionally excluded because
// aenv identifiers may contain them (e.g., log-pretty).
func isWordBoundary(r rune) bool {
	switch r {
	case '.', ' ', '\t',
		'(', ')', '[', ']',
		'+', '*', '/', '%',
		'<', '>', '=', '!',
		'&', '|', ',', '?', ':', ';':
		return true
	}

	return false
}

// wordBounds returns the current word at the cursor position and its byte
// boundaries within input. Words are delimited by whitespace, dots, and
// expr-lang operator/punctuation characters.
// Returns an empty word when the cursor sits on a boundary (after a space,
// between dots, start of line, etc.).
func wordBounds(input string, cursor int) (word string, start, end int) {
	if cursor > len(input) {
		cursor = len(input)
	}

	// Walk backward from cursor to find word start.
	start = cursor

	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(input[:start])
		if isWordBoundary(r) {
			break
		}

		start -= size
	}

	// Walk forward from cursor to find word end.
	end = cursor

	for end < len(input) {
		r, size := utf8.DecodeRuneInString(input[end:])
		if isWordBoundary(r) {
			break
		}

		end += size
	}

	word = input[start:end]

	return word, start, end
}

// parentPath returns the dot-separated prefix path leading up to the current
// word, considering only the contiguous member-access chain. For input
// "x + server.http.ho" with the word "ho", the parent path is "server.http".
// Returns "" for top-level words.
func parentPath(input string, wordStart int) string {
	prefix := input[:wordStart]
	prefix = strings.TrimRight(prefix, ".")

	if prefix == "" {
		return ""
	}

	// Walk backward from the end of the trimmed prefix. Collect characters
	// that are dots or valid identifier characters. Stop at the first
	// non-dot word boundary.
	end := len(prefix)
	pos := end

	for pos > 0 {
		r, size := utf8.DecodeLastRuneInString(prefix[:pos])
		if r == '.' {
			pos -= size

			continue
		}

		if isWordBoundary(r) {
			break
		}

		pos -= size
	}

	result := strings.TrimSpace(prefix[pos:end])
	if result == "" {
		return ""
	}

	return result
}

// childCandidates returns the names that are valid completions for the given
// parent path. For an empty parent, returns all top-level namespace names plus
// the "env" builtin. For a non-empty parent, resolves the namespace and returns
// the names of direct children (tuple elements that are namespaces).
func childCandidates(ast *lang.AST, parent string) []string {
	if parent == "" {
		// Top-level: all namespace names + env builtin.
		var names []string

		for ns := range ast.All() {
			names = append(names, ns.Identifier.LiteralString())
		}

		names = append(names, "env")

		return names
	}

	// Resolve the parent path segment by segment.
	segments := strings.Split(parent, ".")

	// Find the top-level namespace.
	ns, ok := ast.GetNamespace(segments[0])
	if !ok {
		return nil
	}

	// Walk into nested namespaces for remaining segments.
	val := ns.Value

	for _, seg := range segments[1:] {
		val = findChild(val, seg)
		if val == nil {
			return nil
		}
	}

	// Return names of children.
	return childNames(val)
}

// findChild looks up a child namespace by name within a tuple value.
func findChild(v *lang.Value, name string) *lang.Value {
	if v == nil || v.Type != lang.TypeTuple || v.Tuple == nil {
		return nil
	}

	for _, child := range v.Tuple.Values {
		if child.Type == lang.TypeNamespace && child.Namespace != nil {
			if child.Namespace.Identifier.LiteralString() == name {
				return child.Namespace.Value
			}
		}
	}

	return nil
}

// childNames extracts the identifier names of all namespace-typed children
// within a tuple value.
func childNames(v *lang.Value) []string {
	if v == nil || v.Type != lang.TypeTuple || v.Tuple == nil {
		return nil
	}

	var names []string

	for _, child := range v.Tuple.Values {
		if child.Type == lang.TypeNamespace && child.Namespace != nil {
			names = append(names, child.Namespace.Identifier.LiteralString())
		}
	}

	return names
}

// computeMatches calculates the fuzzy match results for the word at the cursor.
// It returns the matches (ranked best-first), the candidate list, and the word
// boundaries. When the current word is empty at the top level, it returns nil
// matches. When the word is empty after a dot (member access), it returns all
// children as matches.
func (m model) computeMatches() (
	matches fuzzy.Matches,
	candidates []string,
	wordStart, wordEnd int,
) {
	input := m.input.Value()
	cursor := m.input.Position()

	word, ws, we := wordBounds(input, cursor)
	wordStart, wordEnd = ws, we

	if m.mode == modeCtrl {
		if word == "" {
			return nil, nil, wordStart, wordEnd
		}

		candidates = ctrlCommands
	} else {
		parent := parentPath(input, wordStart)
		candidates = childCandidates(m.ast, parent)

		// When the word is empty at the top level, don't show completions
		// (allows the hint text to be visible). After a dot, show all children
		// immediately so the user can browse the available members.
		if word == "" {
			if parent == "" || len(candidates) == 0 {
				return nil, nil, wordStart, wordEnd
			}

			// Return all candidates as unfiltered matches.
			matches = make(fuzzy.Matches, len(candidates))
			for i, c := range candidates {
				matches[i] = fuzzy.Match{Str: c, Index: i}
			}

			return matches, candidates, wordStart, wordEnd
		}
	}

	if len(candidates) == 0 {
		return nil, nil, wordStart, wordEnd
	}

	matches = fuzzy.Find(word, candidates)

	return matches, candidates, wordStart, wordEnd
}

// renderCandidateBar builds the single-line completion bar, ellipsized to fit
// within the given terminal width. Each candidate is rendered with its matched
// characters highlighted. The selected candidate (when tabbing) uses the
// selected style.
func renderCandidateBar(
	matches fuzzy.Matches,
	suggIdx int,
	tabActive bool,
	width int,
) string {
	if len(matches) == 0 || width <= 0 {
		return ""
	}

	const sep = "  "

	sepWidth := lipgloss.Width(sep)
	ellipsis := hintStyle.Render("...")
	ellipsisWidth := lipgloss.Width(ellipsis)

	var b strings.Builder

	used := 0

	for i, match := range matches {
		selected := tabActive && i == suggIdx
		rendered := renderCandidate(match, selected)
		candidateWidth := lipgloss.Width(rendered)

		entryWidth := candidateWidth
		if i > 0 {
			entryWidth += sepWidth
		}

		// Check if adding this candidate would exceed width.
		if used+entryWidth+ellipsisWidth > width && i > 0 {
			b.WriteString(sep)
			b.WriteString(ellipsis)

			break
		}

		if i > 0 {
			b.WriteString(sep)
		}

		b.WriteString(rendered)

		used += entryWidth

		// If this is the last candidate, no need to reserve ellipsis space.
		if i == len(matches)-1 {
			break
		}
	}

	return b.String()
}

// renderCandidate renders a single candidate with matched characters
// highlighted.
func renderCandidate(match fuzzy.Match, selected bool) string {
	baseStyle := suggestionStyle
	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Bold(true)

	if selected {
		baseStyle = selectedStyle
		highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("4")).
			Bold(true)
	}

	matchSet := make(map[int]bool, len(match.MatchedIndexes))
	for _, idx := range match.MatchedIndexes {
		matchSet[idx] = true
	}

	var b strings.Builder

	for i, r := range match.Str {
		ch := string(r)
		if matchSet[i] {
			b.WriteString(highlightStyle.Render(ch))
		} else {
			b.WriteString(baseStyle.Render(ch))
		}
	}

	return b.String()
}

// formatPreview generates a preview string for a namespace.
func formatPreview(ns *lang.Namespace) string {
	var sb strings.Builder

	// Show parameters if any
	if len(ns.Parameters) > 0 {
		var params []string

		for _, p := range ns.Parameters {
			if p.Token != nil {
				params = append(params, p.Token.LiteralString())
			}
		}

		sb.WriteString("(" + strings.Join(params, ", ") + ") -> ")
	}

	// Show value preview
	if ns.Value != nil {
		sb.WriteString(formatValuePreview(ns.Value))
	}

	return sb.String()
}

// formatValuePreview generates a short preview of a value.
func formatValuePreview(v *lang.Value) string {
	if v == nil {
		return "<nil>"
	}

	switch v.Type {
	case lang.TypeBoolean, lang.TypeNumber, lang.TypeString:
		if v.Token != nil {
			s := v.Token.LiteralString()
			if len(s) > 40 {
				return s[:37] + "..."
			}

			return s
		}

		return ""

	case lang.TypeIdentifier:
		if v.Token != nil {
			return "-> " + v.Token.LiteralString()
		}

		return ""

	case lang.TypeExpr:
		src := v.ExprSource()
		if len(src) > 40 {
			return "{{ " + src[:37] + "... }}"
		}

		return "{{ " + src + " }}"

	case lang.TypeTuple:
		if v.Tuple != nil {
			return fmt.Sprintf("{ %d items }", len(v.Tuple.Values))
		}

		return "{}"

	case lang.TypeNamespace:
		if v.Namespace != nil {
			return "<nested: " + v.Namespace.Identifier.LiteralString() + ">"
		}

		return "<nested>"

	default:
		return "<unknown>"
	}
}
