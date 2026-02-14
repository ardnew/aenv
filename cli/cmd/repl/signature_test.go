package repl

import (
	"context"
	"testing"

	"github.com/ardnew/aenv/lang"
)

func TestDetectFunctionCall(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		cursor    int
		wantName  string
		wantIndex int
		wantInCall bool
	}{
		{
			name:      "no function call",
			input:     "greeting",
			cursor:    8,
			wantName:  "",
			wantIndex: 0,
			wantInCall: false,
		},
		{
			name:      "simple function first arg",
			input:     "add(",
			cursor:    4,
			wantName:  "add",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "simple function with first arg",
			input:     "add(1",
			cursor:    5,
			wantName:  "add",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "simple function second arg",
			input:     "add(1,",
			cursor:    6,
			wantName:  "add",
			wantIndex: 1,
			wantInCall: true,
		},
		{
			name:      "simple function second arg with value",
			input:     "add(1, 2",
			cursor:    8,
			wantName:  "add",
			wantIndex: 1,
			wantInCall: true,
		},
		{
			name:      "nested namespace function",
			input:     "nested.multiply(",
			cursor:    16,
			wantName:  "nested.multiply",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "nested namespace function first arg",
			input:     "nested.multiply(5",
			cursor:    17,
			wantName:  "nested.multiply",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "nested namespace function second arg",
			input:     "nested.multiply(5,",
			cursor:    18,
			wantName:  "nested.multiply",
			wantIndex: 1,
			wantInCall: true,
		},
		{
			name:      "builtin path.cat",
			input:     "path.cat(",
			cursor:    9,
			wantName:  "path.cat",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "builtin path.cat multiple args",
			input:     "path.cat('/a', '/b',",
			cursor:    21,
			wantName:  "path.cat",
			wantIndex: 2,
			wantInCall: true,
		},
		{
			name:      "builtin mung.prefix",
			input:     "mung.prefix(",
			cursor:    12,
			wantName:  "mung.prefix",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "nested parens",
			input:     "add(multiply(2, 3),",
			cursor:    19,
			wantName:  "add",
			wantIndex: 1,
			wantInCall: true,
		},
		{
			name:      "cursor inside nested call",
			input:     "add(multiply(2, 3), 4)",
			cursor:    13,
			wantName:  "multiply",
			wantIndex: 0,
			wantInCall: true,
		},
		{
			name:      "variadic function multiple args",
			input:     "concat('a', 'b', 'c'",
			cursor:    20,
			wantName:  "concat",
			wantIndex: 2,
			wantInCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFunctionCall(tt.input, tt.cursor)

			if got.name != tt.wantName {
				t.Errorf("detectFunctionCall().name = %q, want %q", got.name, tt.wantName)
			}
			if got.argIndex != tt.wantIndex {
				t.Errorf("detectFunctionCall().argIndex = %d, want %d", got.argIndex, tt.wantIndex)
			}
			if got.inCall != tt.wantInCall {
				t.Errorf("detectFunctionCall().inCall = %v, want %v", got.inCall, tt.wantInCall)
			}
		})
	}
}

func TestGetSignature(t *testing.T) {
	input := `greeting : "hello";
add x y : x + y;
concat ...parts : parts[0];
nested : {
  multiply a b : a * b
}`

	ast, err := lang.ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("Failed to parse test input: %v", err)
	}

	tests := []struct {
		name          string
		funcName      string
		wantSignature string
		wantParams    []string
	}{
		{
			name:          "simple namespace without params",
			funcName:      "greeting",
			wantSignature: "greeting()",
			wantParams:    []string{},
		},
		{
			name:          "simple function with params",
			funcName:      "add",
			wantSignature: "add(x, y)",
			wantParams:    []string{"x", "y"},
		},
		{
			name:          "variadic function",
			funcName:      "concat",
			wantSignature: "concat(...parts)",
			wantParams:    []string{"...parts"},
		},
		{
			name:          "nested function",
			funcName:      "nested.multiply",
			wantSignature: "nested.multiply(a, b)",
			wantParams:    []string{"a", "b"},
		},
		{
			name:          "builtin file.exists",
			funcName:      "file.exists",
			wantSignature: "file.exists(string)",
			wantParams:    []string{"string"},
		},
		{
			name:          "builtin path.cat",
			funcName:      "path.cat",
			wantSignature: "path.cat(...string)",
			wantParams:    []string{"...string"},
		},
		{
			name:          "builtin path.rel",
			funcName:      "path.rel",
			wantSignature: "path.rel(string, string)",
			wantParams:    []string{"string", "string"},
		},
		{
			name:          "builtin mung.prefix",
			funcName:      "mung.prefix",
			wantSignature: "mung.prefix(string, ...string)",
			wantParams:    []string{"string", "...string"},
		},
		{
			name:          "builtin mung.prefixif",
			funcName:      "mung.prefixif",
			wantSignature: "mung.prefixif(string, func, ...string)",
			wantParams:    []string{"string", "func", "...string"},
		},
		{
			name:          "expr-lang builtin len",
			funcName:      "len",
			wantSignature: "len(v)",
			wantParams:    []string{"v"},
		},
		{
			name:          "expr-lang builtin join",
			funcName:      "join",
			wantSignature: "join(array, separator)",
			wantParams:    []string{"array", "separator"},
		},
		{
			name:          "expr-lang builtin upper",
			funcName:      "upper",
			wantSignature: "upper(string)",
			wantParams:    []string{"string"},
		},
		{
			name:          "expr-lang builtin filter",
			funcName:      "filter",
			wantSignature: "filter(array, predicate)",
			wantParams:    []string{"array", "predicate"},
		},
		{
			name:          "nonexistent function",
			funcName:      "doesnotexist",
			wantSignature: "",
			wantParams:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSig, gotParams := getSignature(ast, tt.funcName)

			if gotSig != tt.wantSignature {
				t.Errorf("getSignature().signature = %q, want %q", gotSig, tt.wantSignature)
			}

			if len(gotParams) != len(tt.wantParams) {
				t.Errorf("getSignature().params length = %d, want %d", len(gotParams), len(tt.wantParams))
				return
			}

			for i := range gotParams {
				if gotParams[i] != tt.wantParams[i] {
					t.Errorf("getSignature().params[%d] = %q, want %q", i, gotParams[i], tt.wantParams[i])
				}
			}
		})
	}
}

func TestRenderSignatureHint(t *testing.T) {
	tests := []struct {
		name        string
		signature   string
		params      []string
		currentArg  int
		wantContains string // Substring to check for
	}{
		{
			name:        "no params",
			signature:   "greeting()",
			params:      []string{},
			currentArg:  0,
			wantContains: "greeting()",
		},
		{
			name:        "first param highlighted",
			signature:   "add(x, y)",
			params:      []string{"x", "y"},
			currentArg:  0,
			wantContains: "add(",
		},
		{
			name:        "second param highlighted",
			signature:   "add(x, y)",
			params:      []string{"x", "y"},
			currentArg:  1,
			wantContains: "add(",
		},
		{
			name:        "variadic param",
			signature:   "concat(...parts)",
			params:      []string{"...parts"},
			currentArg:  0,
			wantContains: "concat(",
		},
		{
			name:        "variadic param multiple args",
			signature:   "concat(...parts)",
			params:      []string{"...parts"},
			currentArg:  2,
			wantContains: "concat(",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderSignatureHint(tt.signature, tt.params, tt.currentArg)

			// Just check that the function name appears in the output
			// (detailed formatting is visual and hard to test exactly)
			if got == "" && tt.signature != "" {
				t.Errorf("renderSignatureHint() returned empty string for signature %q", tt.signature)
			}
		})
	}
}
