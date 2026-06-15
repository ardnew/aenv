package repl

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/expr-lang/expr/builtin"
	"github.com/sahilm/fuzzy"

	"github.com/ardnew/aenv/lang"
)

// ctrlCommands are the available control-mode commands.
const cmdHelp = "help"

var ctrlCommands = []string{cmdHelp, "list", "edit", "clear", "quit"}

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
// built-in environment names. For a non-empty parent, resolves the namespace or
// built-in and returns the names of direct children.
func childCandidates(ast *lang.AST, parent string) []string {
	if parent == "" {
		// Top-level: all namespace names + built-in environment.
		builtinEnvKeys := lang.BuiltinEnvKeys()
		exprBuiltins := ExprLangBuiltinNames()
		names := make(
			[]string,
			0,
			len(ast.Namespaces)+len(builtinEnvKeys)+len(exprBuiltins),
		)

		seen := make(map[string]bool, len(ast.Namespaces))

		for ns := range ast.All() {
			if !seen[ns.Name] {
				seen[ns.Name] = true
				names = append(names, ns.Name)
			}
		}

		// Add all built-in environment keys
		names = append(names, builtinEnvKeys...)

		// Add expr-lang builtin functions
		names = append(names, ExprLangBuiltinNames()...)

		return names
	}

	// Resolve the parent path segment by segment.
	segments := strings.Split(parent, ".")

	// First, try to resolve as a namespace in the AST.
	ns, ok := ast.GetNamespace(segments[0])
	if ok {
		// Walk into nested namespaces for remaining segments.
		val := ns.Value

		for _, seg := range segments[1:] {
			val = findChild(val, seg)
			if val == nil {
				break
			}
		}

		if val != nil {
			// Return names of children from the AST.
			return childNames(val)
		}
	}

	// If not found in AST, try the built-in environment.
	return lang.BuiltinEnvLookup(parent)
}

// findChild looks up a child namespace by name within a block value.
func findChild(v *lang.Value, name string) *lang.Value {
	if v == nil || v.Kind != lang.KindBlock {
		return nil
	}

	for _, child := range v.Entries {
		if child.Name == name {
			return child.Value
		}
	}

	return nil
}

// childNames extracts the identifier names of all namespace children
// within a block value.
func childNames(v *lang.Value) []string {
	if v == nil || v.Kind != lang.KindBlock {
		return nil
	}

	names := make([]string, 0, len(v.Entries))

	for _, child := range v.Entries {
		names = append(names, child.Name)
	}

	return names
}

// computeMatches calculates the fuzzy match results for the word at the cursor.
// It returns the matches (ranked best-first), the candidate list, and the word
// boundaries. In command mode, empty input returns all commands; after "help",
// empty input returns all help topics. In expression mode, empty input at the
// top level returns nil (to keep hint text visible) unless inside a function
// call. After a dot, all children are returned immediately.
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
		// When the first word is already "help" (or "h") and the user is
		// typing a second word, offer help topic names as candidates.
		prefix := strings.TrimSpace(input[:ws])
		if prefix == cmdHelp || prefix == "h" {
			candidates = helpTopicNames()
		} else {
			candidates = ctrlCommands
		}

		if word == "" {
			// Return all candidates as unfiltered matches so the user can
			// browse commands (empty input) or help topics (":help ").
			matches = make(fuzzy.Matches, len(candidates))
			for i, c := range candidates {
				matches[i] = fuzzy.Match{Str: c, Index: i}
			}

			return matches, candidates, wordStart, wordEnd
		}
	} else {
		parent := parentPath(input, wordStart)
		candidates = childCandidates(m.ast, parent)

		// When the word is empty at the top level, don't show completions
		// (allows the hint text to be visible) UNLESS we are inside a
		// function call, where we always want candidates available for
		// type-based filtering. After a dot, show all children immediately
		// so the user can browse the available members.
		if word == "" {
			inCall := detectFunctionCall(input, cursor).inCall
			if (parent == "" && !inCall) || len(candidates) == 0 {
				return nil, nil, wordStart, wordEnd
			}

			// Return all candidates as unfiltered matches, priority-sorted.
			matches = make(fuzzy.Matches, len(candidates))
			for i, c := range candidates {
				matches[i] = fuzzy.Match{Str: c, Index: i}
			}

			sortMatchesByPriority(matches, m.ast)

			return matches, candidates, wordStart, wordEnd
		}
	}

	if len(candidates) == 0 {
		return nil, nil, wordStart, wordEnd
	}

	matches = fuzzy.Find(word, candidates)
	sortMatchesByPriority(matches, m.ast)

	return matches, candidates, wordStart, wordEnd
}

// getExpectedParamType returns the type name expected at argIndex for the
// given function. For variadic parameters, the element type applies to all
// subsequent arguments. Returns "" when no type information is available.
func getExpectedParamType(ast *lang.AST, funcName string, argIndex int) string {
	_, params := getSignature(ast, funcName)
	if len(params) == 0 {
		return ""
	}

	if argIndex < len(params) {
		return strings.TrimPrefix(params[argIndex], "...")
	}

	// Beyond declared params: use last if variadic.
	last := params[len(params)-1]
	if after, ok := strings.CutPrefix(last, "..."); ok {
		return after
	}

	return ""
}

// isCallableCandidate returns whether a candidate name represents something
// callable: a parameterized namespace, an expr-lang builtin, or a project
// builtin function.
func isCallableCandidate(name string, ast *lang.AST) bool {
	// Parameterized user-defined namespaces are callable.
	if ns, ok := ast.GetNamespace(name); ok {
		return len(ns.Params) > 0
	}

	// Expr-lang builtins and project builtin functions are callable.
	return isFunction(name)
}

// filterByParamType filters completion matches based on the expected parameter
// type at the current argument position. When the expected type is "predicate"
// or "func", only callable candidates are kept. For all other types (or when
// no type information is available), the matches are returned unchanged.
func filterByParamType(
	matches fuzzy.Matches,
	ast *lang.AST,
	fc functionCall,
) fuzzy.Matches {
	if len(matches) == 0 {
		return matches
	}

	expectedType := getExpectedParamType(ast, fc.name, fc.argIndex)

	// Generic or unknown types: no filtering.
	switch expectedType {
	case "", "v", "arg":
		return matches
	}

	switch expectedType {
	case "predicate", typeNameFunc:
		// Only show callable candidates (parameterized namespaces, builtins).
		filtered := make(fuzzy.Matches, 0, len(matches))

		for _, m := range matches {
			if isCallableCandidate(m.Str, ast) {
				filtered = append(filtered, m)
			}
		}

		if len(filtered) > 0 {
			return filtered
		}
	}

	// For value types or when filtering would empty the list, return all.
	return matches
}

// matchPriority returns the sort priority for a completion candidate name:
//
//	0 — non-parameterised user-defined namespace
//	1 — parameterised user-defined namespace
//	2 — built-in environment or expr-lang builtin
func matchPriority(name string, a *lang.AST) int {
	if ns, ok := a.GetNamespace(name); ok {
		if len(ns.Params) == 0 {
			return 0
		}

		return 1
	}

	return 2
}

// sortMatchesByPriority re-orders matches so that user-defined namespace
// identifiers appear before parameterised namespaces, which in turn appear
// before built-in and expr-lang functions. The original fuzzy-score ordering
// is preserved within each priority band via a stable sort.
func sortMatchesByPriority(matches fuzzy.Matches, a *lang.AST) {
	slices.SortStableFunc(matches, func(x, y fuzzy.Match) int {
		return cmp.Compare(matchPriority(x.Str, a), matchPriority(y.Str, a))
	})
}

// candidateEntry holds the pre-rendered text and display width of one
// completion candidate.
type candidateEntry struct {
	rendered string
	w        int
}

// buildCandidateEntries pre-renders every match.
func buildCandidateEntries(
	matches fuzzy.Matches,
	suggIdx int,
	tabActive bool,
	showCallHint bool,
) []candidateEntry {
	entries := make([]candidateEntry, len(matches))

	for i, match := range matches {
		r := renderCandidate(match, tabActive && i == suggIdx, showCallHint)
		entries[i] = candidateEntry{r, lipgloss.Width(r)}
	}

	return entries
}

// candidateWindowStart returns the smallest start index ≤ suggIdx such that
// the range [start..suggIdx] fits within the given budget.
func candidateWindowStart(
	entries []candidateEntry,
	suggIdx int,
	sepWidth, leftArrowWidth, rightArrowWidth int,
	totalWidth int,
) int {
	for start := range suggIdx {
		leftCost := 0
		if start > 0 {
			leftCost = leftArrowWidth
		}

		budget := totalWidth - leftCost - rightArrowWidth
		needed := 0

		for i := start; i <= suggIdx; i++ {
			if i > start {
				needed += sepWidth
			}

			needed += entries[i].w
		}

		if needed <= budget {
			return start
		}
	}

	return suggIdx
}

// candidateWindowEnd returns the last index reachable from windowStart within
// budget, pre-computing whether a right-arrow is required.
func candidateWindowEnd(
	entries []candidateEntry,
	windowStart int,
	sepWidth, rightArrowWidth int,
	budget int,
) int {
	used := 0
	windowEnd := windowStart - 1

	for i := windowStart; i < len(entries); i++ {
		extra := entries[i].w
		if i > windowStart {
			extra += sepWidth
		}

		rightReserve := 0
		if i < len(entries)-1 {
			rightReserve = rightArrowWidth
		}

		if used+extra+rightReserve > budget {
			break
		}

		used += extra
		windowEnd = i
	}

	// Guarantee the selected item is always shown even if it alone exceeds
	// the terminal width.
	if windowEnd < windowStart {
		return windowStart
	}

	return windowEnd
}

// renderCandidateBar builds the single-line completion bar that fits within
// the given terminal width. Each candidate is rendered with its matched
// characters highlighted. The selected candidate (when tabbing) uses the
// selected style.
//
// When the full candidate list does not fit on one line the bar scrolls
// horizontally so that the selected candidate is always visible. A "← "
// prefix is shown when candidates are hidden to the left, and a " →" suffix
// is shown when candidates are hidden to the right.
func renderCandidateBar(
	matches fuzzy.Matches,
	suggIdx int,
	tabActive bool,
	width int,
	showCallHint ...bool,
) string {
	if len(matches) == 0 || width <= 0 {
		return ""
	}

	hint := len(showCallHint) == 0 || showCallHint[0]

	const sep = "  "

	sepWidth := lipgloss.Width(sep)

	leftArrow := hintStyle.Render("← ")
	rightArrow := hintStyle.Render(" →")
	leftArrowWidth := lipgloss.Width(leftArrow)
	rightArrowWidth := lipgloss.Width(rightArrow)

	entries := buildCandidateEntries(matches, suggIdx, tabActive, hint)

	// Determine the visible window.
	windowStart := 0

	if tabActive && suggIdx > 0 {
		windowStart = candidateWindowStart(
			entries, suggIdx,
			sepWidth, leftArrowWidth, rightArrowWidth,
			width,
		)
	}

	needLeft := windowStart > 0

	budget := width
	if needLeft {
		budget -= leftArrowWidth
	}

	windowEnd := candidateWindowEnd(
		entries, windowStart,
		sepWidth, rightArrowWidth,
		budget,
	)

	needRight := windowEnd < len(entries)-1

	var b strings.Builder

	if needLeft {
		b.WriteString(leftArrow)
	}

	for i := windowStart; i <= windowEnd; i++ {
		if i > windowStart {
			b.WriteString(sep)
		}

		b.WriteString(entries[i].rendered)
	}

	if needRight {
		b.WriteString(rightArrow)
	}

	return b.String()
}

// renderCandidate renders a single candidate with matched characters
// highlighted. When showCallHint is true, functions are displayed with
// a "()" suffix.
func renderCandidate(match fuzzy.Match, selected, showCallHint bool) string {
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

	// Add "()" suffix for functions (not applied to actual completion)
	if showCallHint && isFunction(match.Str) {
		b.WriteString(baseStyle.Render("()"))
	}

	return b.String()
}

// formatPreview generates a preview string for a namespace.
func formatPreview(ns *lang.Namespace) string {
	var sb strings.Builder

	// Show parameters if any
	if len(ns.Params) > 0 {
		params := make([]string, 0, len(ns.Params))

		for _, p := range ns.Params {
			paramName := p.Name
			if p.Variadic {
				paramName = "..." + paramName
			}

			params = append(params, paramName)
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

	switch v.Kind {
	case lang.KindExpr:
		src := v.Source
		if len(src) > 40 {
			return src[:37] + "..."
		}

		return src

	case lang.KindBlock:
		return fmt.Sprintf("{ %d items }", len(v.Entries))

	default:
		return "<unknown>"
	}
}

// tryTabFinish checks whether the word at the cursor is an exact candidate
// match, and if so returns the contextual suffix to insert. The second return
// value is false when tab-finish does not apply (word is empty, cursor is not
// at the end of the word, word is not a candidate, etc.).
func (m model) tryTabFinish() (string, bool) {
	input := m.input.Value()
	cursor := m.input.Position()

	word, ws, we := wordBounds(input, cursor)
	if word == "" || cursor != we {
		return "", false
	}

	// Don't tab-finish when the next character indicates continuation
	// (e.g., dot for member access, opening paren for existing call,
	// closing paren for argument boundary, comma between arguments).
	if cursor < len(input) {
		next, _ := utf8.DecodeRuneInString(input[cursor:])
		if next != ' ' && next != '\t' {
			return "", false
		}
	}

	var cands []string

	if m.mode == modeCtrl {
		prefix := strings.TrimSpace(input[:ws])
		if prefix == cmdHelp || prefix == "h" {
			cands = helpTopicNames()
		} else {
			cands = ctrlCommands
		}
	} else {
		parent := parentPath(input, ws)
		cands = childCandidates(m.ast, parent)
	}

	if !slices.Contains(cands, word) {
		return "", false
	}

	// Command-mode completions are never callable — always space-separated.
	if m.mode == modeCtrl {
		return " ", true
	}

	return tabFinishSuffix(m.ast, input, cursor, word), true
}

// tabFinishSuffix returns the contextual text to insert when Tab-finishing on
// a word that already matches a candidate exactly.
//
// Context rules:
//   - Outside a function call: " " for non-callable, "(" for callable
//   - Inside a function call, non-final argument: ", "
//   - Inside a function call, variadic argument: ", "
//   - Inside a function call, final non-variadic argument: ") "
func tabFinishSuffix(
	ast *lang.AST,
	input string,
	cursor int,
	candidateName string,
) string {
	fc := detectFunctionCall(input, cursor)
	if !fc.inCall {
		if isCallableCandidate(candidateName, ast) {
			return "("
		}

		return " "
	}

	// Inside a function call — determine argument finality.
	_, params := getSignature(ast, fc.name)
	if len(params) == 0 {
		// Unknown or zero-arg signature; default to ", ".
		return ", "
	}

	lastParam := params[len(params)-1]
	isVariadic := strings.HasPrefix(lastParam, "...")

	// Final argument: at or beyond the last param, and not variadic.
	if fc.argIndex >= len(params)-1 && !isVariadic {
		return ") "
	}

	return ", "
}

// isFunction checks if a name refers to a function that should display with
// "()".
// This includes expr-lang builtins and builtin environment functions that are
// callable (not simple values or namespaces).
func isFunction(name string) bool {
	// Check expr-lang builtins using the builtin.Index map
	if _, ok := builtin.Index[name]; ok {
		return true
	}

	// Check builtin environment functions
	if name == "cwd" {
		return true // cwd is a function
	}

	// Check nested builtins (e.g., "file.exists" won't appear as top-level)
	// For top-level display, we only check simple names
	return false
}
