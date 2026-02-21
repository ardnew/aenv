package lang

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFormat_Simple(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		indent int
		want   string
	}{
		{
			name:   "single namespace",
			input:  `x : 1`,
			indent: 0,
			want:   "x : 1;\n",
		},
		{
			name:   "multiple namespaces",
			input:  `x : 1; y : 2`,
			indent: 0,
			want:   "x : 1; y : 2;\n",
		},
		{
			name:   "with parameters",
			input:  `greet name : "Hello, " + name`,
			indent: 0,
			want:   `greet name : "Hello, " + name;` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			var buf bytes.Buffer
			if err := ast.Format(context.Background(), &buf, tt.indent); err != nil {
				t.Fatalf("format error: %v", err)
			}

			got := buf.String()
			if got != tt.want {
				t.Errorf("format mismatch:\nwant: %q\ngot:  %q", tt.want, got)
			}
		})
	}
}

func TestFormat_Block(t *testing.T) {
	input := `config : { host : "localhost"; port : 8080 }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Compact format (indent=0)
	var compact bytes.Buffer
	if err := ast.Format(context.Background(), &compact, 0); err != nil {
		t.Fatalf("format error: %v", err)
	}

	compactStr := compact.String()
	if !strings.Contains(compactStr, "config : {") {
		t.Errorf("compact format should contain block")
	}

	// Pretty format (indent=2)
	var pretty bytes.Buffer
	if err := ast.Format(context.Background(), &pretty, 2); err != nil {
		t.Fatalf("format error: %v", err)
	}

	prettyStr := pretty.String()
	if !strings.Contains(prettyStr, "config : {\n") {
		t.Errorf("pretty format should have newlines in block")
	}
}

func TestFormat_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple",
			input: `x : 1`,
		},
		{
			name:  "with params",
			input: `greet name : "Hello, " + name`,
		},
		{
			name:  "block",
			input: `config : { host : "localhost"; port : 8080 }`,
		},
		{
			name:  "multiple",
			input: `x : 1; y : 2; z : 3`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse original
			ast1, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			// Format to string
			var buf bytes.Buffer
			if err := ast1.Format(context.Background(), &buf, 0); err != nil {
				t.Fatalf("format error: %v", err)
			}

			formatted := strings.TrimSpace(buf.String())

			// Parse formatted
			ast2, err := ParseString(context.Background(), formatted)
			if err != nil {
				t.Fatalf("parse formatted error: %v\nformatted: %s", err, formatted)
			}

			// Should have same number of namespaces
			if len(ast1.Namespaces) != len(ast2.Namespaces) {
				t.Errorf("namespace count mismatch: %d vs %d", len(ast1.Namespaces), len(ast2.Namespaces))
			}

			// Check each namespace
			for i := range ast1.Namespaces {
				if ast1.Namespaces[i].Name != ast2.Namespaces[i].Name {
					t.Errorf("namespace[%d] name mismatch: %q vs %q",
						i, ast1.Namespaces[i].Name, ast2.Namespaces[i].Name)
				}
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	input := `x : 1; y : "hello"`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := ast.FormatJSON(context.Background(), &buf, 2); err != nil {
		t.Fatalf("format JSON error: %v", err)
	}

	jsonStr := buf.String()
	if !strings.Contains(jsonStr, `"x"`) {
		t.Errorf("JSON should contain x key")
	}
	if !strings.Contains(jsonStr, `"y"`) {
		t.Errorf("JSON should contain y key")
	}
}

func TestFormatYAML(t *testing.T) {
	input := `x : 1; y : "hello"`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := ast.FormatYAML(context.Background(), &buf, 2); err != nil {
		t.Fatalf("format YAML error: %v", err)
	}

	yamlStr := buf.String()
	// YAML key names may or may not have quotes, so just check for the key presence
	if !strings.Contains(yamlStr, "x") {
		t.Errorf("YAML should contain x key, got: %s", yamlStr)
	}
	if !strings.Contains(yamlStr, "hello") || !strings.Contains(yamlStr, "y") {
		t.Errorf("YAML should contain y key with value hello, got: %s", yamlStr)
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "nil",
			input: nil,
			want:  "nil",
		},
		{
			name:  "bool true",
			input: true,
			want:  "true",
		},
		{
			name:  "bool false",
			input: false,
			want:  "false",
		},
		{
			name:  "int",
			input: 42,
			want:  "42",
		},
		{
			name:  "int64",
			input: int64(42),
			want:  "42",
		},
		{
			name:  "float64",
			input: 3.14,
			want:  "3.14",
		},
		{
			name:  "string",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "empty array",
			input: []any{},
			want:  "[]",
		},
		{
			name:  "array with values",
			input: []any{1, "two", true},
			want:  `[1, "two", true]`,
		},
		{
			name:  "empty map",
			input: map[string]any{},
			want:  "{}",
		},
		{
			name:  "map with values",
			input: map[string]any{"x": 1, "y": "hello"},
			want:  `{"x": 1, "y": "hello"}`,
		},
		{
			name:  "FuncRef with signature",
			input: &FuncRef{Name: "add", Signature: "add(x, y)"},
			want:  "<func: add(x, y)>",
		},
		{
			name:  "FuncRef name only",
			input: &FuncRef{Name: "cwd", Signature: ""},
			want:  "<func: cwd>",
		},
		{
			name:  "map with function values",
			input: map[string]any{"prefix": func(string) string { return "" }},
			want:  `{"prefix": <func: prefix(_)>}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatResult(tt.input)

			if got != tt.want {
				t.Errorf("FormatResult() = %q, want %q", got, tt.want)
			}
		})
	}
}
