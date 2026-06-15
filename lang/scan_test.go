package lang

import (
	"slices"
	"testing"
)

func TestScanIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected []string
	}{
		{
			name:     "simple addition",
			source:   "x + y",
			expected: []string{"x", "y"},
		},
		{
			name:     "string_literal_skipped",
			source:   `greeting + " world"`,
			expected: []string{"greeting"},
		},
		{
			name:     "function_call",
			source:   "add(x, y)",
			expected: []string{"add", "x", "y"},
		},
		{
			name:     "member_access",
			source:   "config.host",
			expected: []string{"config"},
		},
		{
			name:     "keyword_skipped",
			source:   "x > 0 and y < 10",
			expected: []string{"x", "y"},
		},
		{
			name:     "hyphenated",
			source:   "log-level",
			expected: []string{"log-level"},
		},
		{
			name:     "number_skipped",
			source:   "x + 42",
			expected: []string{"x"},
		},
		{
			name:     "boolean_skipped",
			source:   "x == true",
			expected: []string{"x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanIdentifiers(tt.source)
			slices.Sort(result)
			slices.Sort(tt.expected)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("scanIdentifiers(%q) = %v, want %v",
					tt.source, result, tt.expected)
			}
		})
	}
}
