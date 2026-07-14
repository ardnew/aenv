package cli

import (
	"cmp"
	"strings"
	"unicode/utf8"
)

// clamp constrains v to the inclusive range [lo, hi].
func clamp[T cmp.Ordered](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// runeCount returns the number of runes in s.
func runeCount(s string) int { return utf8.RuneCountInString(s) }

// lineCount returns the number of newline-delimited lines in text.
func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// joinInput flattens one or more lines of value into a single space-delimited
// line, suitable for display in a single-line editor.
func joinInput(value ...string) string {
	return strings.ReplaceAll(strings.Join(value, " "), "\n", " ")
}

// splitInput splits value into its newline-delimited lines.
func splitInput(value string) []string {
	if value == "" {
		return []string{""}
	}
	return strings.Split(value, "\n")
}
