package lang

import (
	"context"
	"testing"
)

func TestParseString_Simple(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // number of namespaces
	}{
		{
			name:  "single namespace",
			input: `greeting : "Hello, World!"`,
			want:  1,
		},
		{
			name:  "multiple namespaces",
			input: `x : 1; y : 2; z : 3`,
			want:  3,
		},
		{
			name:  "with newlines as separators",
			input: "x : 1\ny : 2\nz : 3",
			want:  1, // newlines alone don't separate - need semicolon or comma
		},
		{
			name:  "empty block",
			input: `config : {}`,
			want:  1,
		},
		{
			name:  "block with entries",
			input: `config : { host : "localhost"; port : 8080 }`,
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(ast.Namespaces) != tt.want {
				t.Errorf("expected %d namespaces, got %d", tt.want, len(ast.Namespaces))
			}
		})
	}
}

func TestParseString_Parameters(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		nsName     string
		paramCount int
		variadic   bool
	}{
		{
			name:       "single parameter",
			input:      `greet name : "Hello, " + name`,
			nsName:     "greet",
			paramCount: 1,
			variadic:   false,
		},
		{
			name:       "multiple parameters",
			input:      `add x y : x + y`,
			nsName:     "add",
			paramCount: 2,
			variadic:   false,
		},
		{
			name:       "variadic parameter",
			input:      `sum ...nums : nums[0] + nums[1]`,
			nsName:     "sum",
			paramCount: 1,
			variadic:   true,
		},
		{
			name:       "mixed parameters",
			input:      `func a b ...rest : a + b`,
			nsName:     "func",
			paramCount: 3,
			variadic:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			ns, found := ast.GetNamespace(tt.nsName)
			if !found {
				t.Fatalf("namespace %q not found", tt.nsName)
			}

			if len(ns.Params) != tt.paramCount {
				t.Errorf("expected %d parameters, got %d", tt.paramCount, len(ns.Params))
			}

			if tt.variadic {
				lastParam := ns.Params[len(ns.Params)-1]
				if !lastParam.Variadic {
					t.Errorf("expected last parameter to be variadic")
				}
			}
		})
	}
}

func TestParseString_Blocks(t *testing.T) {
	input := `
		config : {
			host : "localhost";
			port : 8080;
			debug : true
		}
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ns, found := ast.GetNamespace("config")
	if !found {
		t.Fatalf("namespace 'config' not found")
	}

	if ns.Value.Kind != KindBlock {
		t.Fatalf("expected block, got %v", ns.Value.Kind)
	}

	if len(ns.Value.Entries) != 3 {
		t.Errorf("expected 3 entries in block, got %d", len(ns.Value.Entries))
	}

	// Check entries
	expected := map[string]bool{
		"host":  false,
		"port":  false,
		"debug": false,
	}

	for _, entry := range ns.Value.Entries {
		if _, ok := expected[entry.Name]; !ok {
			t.Errorf("unexpected entry: %s", entry.Name)
		}
		expected[entry.Name] = true
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing entry: %s", name)
		}
	}
}

func TestParseString_NestedBlocks(t *testing.T) {
	input := `
		app : {
			server : {
				host : "localhost";
				port : 8080
			};
			database : {
				host : "db.local";
				port : 5432
			}
		}
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ns, found := ast.GetNamespace("app")
	if !found {
		t.Fatalf("namespace 'app' not found")
	}

	if ns.Value.Kind != KindBlock {
		t.Fatalf("expected block, got %v", ns.Value.Kind)
	}

	// Check server block
	var serverEntry *Namespace
	for _, entry := range ns.Value.Entries {
		if entry.Name == "server" {
			serverEntry = entry
			break
		}
	}

	if serverEntry == nil {
		t.Fatalf("server entry not found")
	}

	if serverEntry.Value.Kind != KindBlock {
		t.Errorf("expected server to be a block")
	}

	if len(serverEntry.Value.Entries) != 2 {
		t.Errorf("expected 2 entries in server block, got %d", len(serverEntry.Value.Entries))
	}
}

func TestParseString_Comments(t *testing.T) {
	input := `
		// This is a comment
		x : 1; // inline comment
		// Another comment
		y : 2
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(ast.Namespaces))
	}
}

func TestParseString_Expressions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		nsName string
		kind   ValueKind
	}{
		{
			name:   "simple expression",
			input:  `x : 1 + 2`,
			nsName: "x",
			kind:   KindExpr,
		},
		{
			name:   "string concatenation",
			input:  `msg : "Hello, " + "World"`,
			nsName: "msg",
			kind:   KindExpr,
		},
		{
			name:   "function call",
			input:  `home : env("HOME")`,
			nsName: "home",
			kind:   KindExpr,
		},
		{
			name:   "array literal",
			input:  `nums : [1, 2, 3]`,
			nsName: "nums",
			kind:   KindExpr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			ns, found := ast.GetNamespace(tt.nsName)
			if !found {
				t.Fatalf("namespace %q not found", tt.nsName)
			}

			if ns.Value.Kind != tt.kind {
				t.Errorf("expected kind %v, got %v", tt.kind, ns.Value.Kind)
			}
		})
	}
}

func TestParseString_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing colon",
			input: `x 1`,
		},
		{
			name:  "unclosed block",
			input: `config : { host : "localhost"`,
		},
		{
			name:  "invalid identifier",
			input: `123invalid : 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(context.Background(), tt.input)
			if err == nil {
				t.Errorf("expected parse error, got nil")
			}
		})
	}
}
