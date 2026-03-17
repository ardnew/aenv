package lang

import (
	"context"
	"errors"
	"testing"
)

func TestEvaluateNamespace_Simple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		nsName   string
		expected any
	}{
		{
			name:     "string literal",
			input:    `greeting : "Hello, World!"`,
			nsName:   "greeting",
			expected: "Hello, World!",
		},
		{
			name:     "integer",
			input:    `answer : 42`,
			nsName:   "answer",
			expected: int64(42),
		},
		{
			name:     "boolean true",
			input:    `flag : true`,
			nsName:   "flag",
			expected: true,
		},
		{
			name:     "boolean false",
			input:    `flag : false`,
			nsName:   "flag",
			expected: false,
		},
		{
			name:     "float",
			input:    `pi : 3.14`,
			nsName:   "pi",
			expected: 3.14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), tt.nsName, nil)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			// Handle type conversions (expr-lang may return int or int64)
			switch expected := tt.expected.(type) {
			case int64:
				switch v := result.(type) {
				case int:
					if int64(v) != expected {
						t.Errorf("expected %v (%T), got %v (%T)", expected, expected, v, v)
					}
				case int64:
					if v != expected {
						t.Errorf("expected %v (%T), got %v (%T)", expected, expected, v, v)
					}
				default:
					t.Errorf("expected int64, got %T: %v", result, result)
				}
			default:
				if result != tt.expected {
					t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
				}
			}
		})
	}
}

func TestEvaluateNamespace_Arithmetic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		nsName   string
		expected int
	}{
		{
			name:     "addition",
			input:    `x : 1 + 2`,
			nsName:   "x",
			expected: 3,
		},
		{
			name:     "subtraction",
			input:    `x : 10 - 3`,
			nsName:   "x",
			expected: 7,
		},
		{
			name:     "multiplication",
			input:    `x : 6 * 7`,
			nsName:   "x",
			expected: 42,
		},
		{
			name:     "division",
			input:    `x : 20 / 4`,
			nsName:   "x",
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), tt.nsName, nil)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			// expr-lang may return int or int64 or float64 for division
			switch v := result.(type) {
			case int:
				if v != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, v)
				}
			case int64:
				if int(v) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, v)
				}
			case float64:
				if int(v) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, int(v))
				}
			default:
				t.Errorf("expected int, got %T: %v", result, result)
			}
		})
	}
}

func TestEvaluateNamespace_Block(t *testing.T) {
	input := `config : { host : "localhost"; port : 8080 }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "config", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	if m["host"] != "localhost" {
		t.Errorf("expected host='localhost', got %v", m["host"])
	}

	// port could be int or int64
	switch port := m["port"].(type) {
	case int:
		if port != 8080 {
			t.Errorf("expected port=8080, got %v", port)
		}
	case int64:
		if port != 8080 {
			t.Errorf("expected port=8080, got %v", port)
		}
	default:
		t.Errorf("expected port to be int or int64, got %T: %v", m["port"], m["port"])
	}
}

func TestEvaluateNamespace_BlockSiblingReference(t *testing.T) {
	input := `config : { port : 8080; url : "http://localhost:" + string(port) }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "config", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	expected := "http://localhost:8080"
	if m["url"] != expected {
		t.Errorf("expected url=%q, got %v", expected, m["url"])
	}
}

func TestEvaluateNamespace_WithParameters(t *testing.T) {
	input := `greet name : "Hello, " + name`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "greet", []string{"Alice"})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	expected := "Hello, Alice"
	if result != expected {
		t.Errorf("expected %q, got %v", expected, result)
	}
}

func TestEvaluateNamespace_MultipleParameters(t *testing.T) {
	input := `add x y : x + y`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "add", []string{"10", "32"})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	// Result should be 42 (10 + 32)
	switch v := result.(type) {
	case int:
		if v != 42 {
			t.Errorf("expected 42, got %v", v)
		}
	case int64:
		if v != 42 {
			t.Errorf("expected 42, got %v", v)
		}
	default:
		t.Errorf("expected int, got %T: %v", result, result)
	}
}

func TestEvaluateNamespace_Variadic(t *testing.T) {
	input := `sum ...nums : nums[0] + nums[1] + nums[2]`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "sum", []string{"1", "2", "3"})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	// Result should be 6 (1 + 2 + 3)
	switch v := result.(type) {
	case int:
		if v != 6 {
			t.Errorf("expected 6, got %v", v)
		}
	case int64:
		if v != 6 {
			t.Errorf("expected 6, got %v", v)
		}
	default:
		t.Errorf("expected int, got %T: %v", result, result)
	}
}

func TestEvaluateNamespace_EnvMap(t *testing.T) {
	input := `home : env.HOME`

	processEnv := []string{"HOME=/home/testuser"}

	ast, err := ParseString(context.Background(), input, WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "home", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	expected := "/home/testuser"
	if result != expected {
		t.Errorf("expected %q, got %v", expected, result)
	}
}

func TestEvaluateNamespace_NamespaceReference(t *testing.T) {
	input := `
		port : 8080;
		server : "localhost:" + string(port)
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "server", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	expected := "localhost:8080"
	if result != expected {
		t.Errorf("expected %q, got %v", expected, result)
	}
}

func TestEvaluateNamespace_BuiltinFunctions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		nsName string
		check  func(t *testing.T, result any)
	}{
		{
			name:   "cwd",
			input:  `dir : fs.cwd()`,
			nsName: "dir",
			check: func(t *testing.T, result any) {
				if _, ok := result.(string); !ok {
					t.Errorf("expected string, got %T", result)
				}
			},
		},
		{
			name:   "platform",
			input:  `plat : platform.os`,
			nsName: "plat",
			check: func(t *testing.T, result any) {
				if _, ok := result.(string); !ok {
					t.Errorf("expected string, got %T", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), tt.nsName, nil)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			tt.check(t, result)
		})
	}
}

func TestEvaluateExpr_Simple(t *testing.T) {
	ast := &AST{}

	tests := []struct {
		name     string
		expr     string
		expected any
	}{
		{
			name:     "literal number",
			expr:     "42",
			expected: 42,
		},
		{
			name:     "literal string",
			expr:     `"hello"`,
			expected: "hello",
		},
		{
			name:     "arithmetic",
			expr:     "10 + 5",
			expected: 15,
		},
		{
			name:     "boolean",
			expr:     "true",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ast.EvaluateExpr(context.Background(), tt.expr)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			switch expected := tt.expected.(type) {
			case int:
				// Handle int/int64 conversion
				switch v := result.(type) {
				case int:
					if v != expected {
						t.Errorf("expected %v, got %v", expected, v)
					}
				case int64:
					if int(v) != expected {
						t.Errorf("expected %v, got %v", expected, v)
					}
				default:
					t.Errorf("expected int, got %T: %v", result, result)
				}
			default:
				if result != expected {
					t.Errorf("expected %v, got %v", expected, result)
				}
			}
		})
	}
}

func TestEvaluateNamespace_ParameterCountMismatch(t *testing.T) {
	input := `greet name : "Hello, " + name`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Too few arguments
	_, err = ast.EvaluateNamespace(context.Background(), "greet", nil)
	if err == nil {
		t.Error("expected error for missing parameter, got nil")
	}
}

func TestEvaluateNamespace_ExtraArgsIgnored(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		nsName   string
		args     []string
		expected any
	}{
		{
			name:     "one_param_two_args",
			input:    `greet name : "Hello, " + name`,
			nsName:   "greet",
			args:     []string{"Alice", "Bob"},
			expected: "Hello, Alice",
		},
		{
			name:     "no_param_extra_arg",
			input:    `x : 42`,
			nsName:   "x",
			args:     []string{"extra"},
			expected: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), tt.nsName, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateNamespace_NotDefined(t *testing.T) {
	input := `x : 1`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = ast.EvaluateNamespace(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for undefined namespace, got nil")
	}
}

func TestEvaluateExpr_FunctionSignatureInError(t *testing.T) {
	input := `
		add x y : x + y;
		greet name : "Hello, " + name;
		join ...parts : parts
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tests := []struct {
		name              string
		expr              string
		wantSignatureHint string // Function signature we expect in the error
	}{
		{
			name:              "too few arguments",
			expr:              "add(1)",
			wantSignatureHint: "add(x, y)",
		},
		{
			name:              "too many arguments",
			expr:              "greet('Alice', 'Bob')",
			wantSignatureHint: "greet(name)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ast.EvaluateExpr(context.Background(), tt.expr)
			if err == nil {
				t.Error("expected error for incorrect argument count, got nil")
				return
			}

			// Check if error contains signature
			var langErr *Error
			if !errors.As(err, &langErr) {
				t.Fatalf("expected *lang.Error, got %T", err)
			}

			// Convert to LogValue to check attributes
			logVal := langErr.LogValue()
			attrs := logVal.Group()

			foundSig := false
			for _, attr := range attrs {
				if attr.Key == "signature" {
					sig := attr.Value.String()
					if sig != tt.wantSignatureHint {
						t.Errorf("expected signature %q, got %q", tt.wantSignatureHint, sig)
					}
					foundSig = true
					break
				}
			}

			if !foundSig {
				t.Errorf("error should contain function signature, got: %v", err)
			}
		})
	}
}

func TestEvaluateExpr_BuiltinAsVariable(t *testing.T) {
	t.Run("bare builtin returns FuncRef", func(t *testing.T) {
		names := []string{"now", "len", "upper"}
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				ast, err := ParseString(context.Background(), "")
				if err != nil {
					t.Fatalf("parse error: %v", err)
				}

				result, err := ast.EvaluateExpr(context.Background(), name)
				if err != nil {
					t.Fatalf("evaluate error: %v", err)
				}

				ref, ok := result.(*FuncRef)
				if !ok {
					t.Fatalf("expected *FuncRef, got %T (%v)", result, result)
				}

				if ref.Name != name {
					t.Errorf("Name: expected %q, got %q", name, ref.Name)
				}
			})
		}
	})

	t.Run("let binding calls builtin", func(t *testing.T) {
		tests := []struct {
			name   string
			source string
			want   any
		}{
			{
				name:   "upper via let",
				source: `let x = upper; x("hello")`,
				want:   "HELLO",
			},
			{
				name:   "len via let",
				source: `let x = len; x("abc")`,
				want:   3,
			},
		}

		ast, err := ParseString(context.Background(), "")
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := ast.EvaluateExpr(context.Background(), tt.source)
				if err != nil {
					t.Fatalf("evaluate error: %v", err)
				}

				// expr-lang may return int or int64; compare loosely.
				switch want := tt.want.(type) {
				case string:
					if result != want {
						t.Errorf("expected %q, got %v (%T)", want, result, result)
					}
				case int:
					switch got := result.(type) {
					case int:
						if got != want {
							t.Errorf("expected %d, got %d", want, got)
						}
					case int64:
						if got != int64(want) {
							t.Errorf("expected %d, got %d", want, got)
						}
					default:
						t.Errorf("expected int-like, got %T (%v)", result, result)
					}
				default:
					t.Errorf("unhandled want type %T", tt.want)
				}
			})
		}
	})
}

func TestEvaluateExpr_FunctionIdentifier_ReturnsFuncRef(t *testing.T) {
	input := `add x y : x + y`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Evaluating a function identifier should return a *FuncRef, not nil.
	result, err := ast.EvaluateExpr(context.Background(), "add")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	ref, ok := result.(*FuncRef)
	if !ok {
		t.Fatalf("expected *FuncRef, got %T (%v)", result, result)
	}

	if ref.Name != "add" {
		t.Errorf("Name: expected %q, got %q", "add", ref.Name)
	}

	if ref.Signature != "add(x, y)" {
		t.Errorf("Signature: expected %q, got %q", "add(x, y)", ref.Signature)
	}
}

func TestEvaluateExpr_NamespaceStoresFunction(t *testing.T) {
	tests := []struct {
		name   string
		input  string // aenv source
		expr   string // expression to evaluate
		check  func(t *testing.T, result any)
	}{
		{
			name:  "namespace stores upper",
			input: `mynow : now`,
			expr:  `mynow()`,
			check: func(t *testing.T, result any) {
				t.Helper()
				if result == nil {
					t.Fatal("expected non-nil time result")
				}
			},
		},
		{
			name:  "namespace stores upper function",
			input: `up : upper`,
			expr:  `up("hello")`,
			check: func(t *testing.T, result any) {
				t.Helper()
				if result != "HELLO" {
					t.Errorf("expected %q, got %v", "HELLO", result)
				}
			},
		},
		{
			name:  "namespace stores user-defined function",
			input: `add x y : x + y; a : add`,
			expr:  `a(1, 2)`,
			check: func(t *testing.T, result any) {
				t.Helper()
				switch v := result.(type) {
				case int:
					if v != 3 {
						t.Errorf("expected 3, got %d", v)
					}
				case int64:
					if v != 3 {
						t.Errorf("expected 3, got %d", v)
					}
				default:
					t.Errorf("expected int-like, got %T (%v)", result, result)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateExpr(context.Background(), tt.expr)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			tt.check(t, result)
		})
	}
}

func TestEvaluateExpr_FunctionPassthrough(t *testing.T) {
	t.Run("fs.cwd stored in namespace", func(t *testing.T) {
		ast, err := ParseString(context.Background(), `f : fs.cwd`)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}

		result, err := ast.EvaluateExpr(context.Background(), `f()`)
		if err != nil {
			t.Fatalf("evaluate error: %v", err)
		}

		s, ok := result.(string)
		if !ok {
			t.Fatalf("expected string, got %T (%v)", result, result)
		}

		if s == "" {
			t.Error("expected non-empty cwd string")
		}
	})

	t.Run("mung stored in namespace returns FuncRef", func(t *testing.T) {
		ast, err := ParseString(context.Background(), `f : mung`)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}

		result, err := ast.EvaluateExpr(context.Background(), `f`)
		if err != nil {
			t.Fatalf("evaluate error: %v", err)
		}

		ref, ok := result.(*FuncRef)
		if !ok {
			t.Fatalf("expected *FuncRef, got %T (%v)", result, result)
		}

		if ref.Name != "f" {
			t.Errorf("Name: expected %q, got %q", "f", ref.Name)
		}
	})

	t.Run("function inside a block", func(t *testing.T) {
		ast, err := ParseString(context.Background(), `id : { up : upper }`)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}

		result, err := ast.EvaluateExpr(context.Background(), `id.up("hi")`)
		if err != nil {
			t.Fatalf("evaluate error: %v", err)
		}

		if result != "HI" {
			t.Errorf("expected %q, got %v", "HI", result)
		}
	})
}

func TestValidateNamespaces(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid namespaces",
			input:     `a : 1; b : 2; c : a + b`,
			wantError: false,
		},
		{
			name:      "undefined identifier",
			input:     `x : undefined_var`,
			wantError: true,
		},
		{
			name:      "invalid expression in block",
			input:     `bad : { nested : unknown }`,
			wantError: true,
		},
		{
			name:      "parameterized namespace should be skipped",
			input:     `func x y : x + y`,
			wantError: false,
		},
		{
			name:      "mix of valid and parameterized",
			input:     `val : 42; func x : val * x`,
			wantError: false,
		},
		{
			name:      "error in non-parameterized with valid parameterized",
			input:     `bad : undefined; func x : x + 1`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			err = ast.ValidateNamespaces(context.Background())
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else {
					// Verify it's wrapped with ErrValidation
					var validationErr *Error
					if !errors.As(err, &validationErr) {
						t.Errorf("expected *Error type, got %T", err)
					} else if !errors.Is(err, ErrValidation) {
						t.Errorf("expected error wrapping ErrValidation, got %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEvaluateNamespace_BlockShadowing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		nsName   string
		expected map[string]any
	}{
		{
			name:   "block entry override",
			input:  `config : { a : 1; b : 2 }; config : { a : 3 }`,
			nsName: "config",
			expected: map[string]any{
				"a": 3,
				"b": 2,
			},
		},
		{
			name:   "block entry addition",
			input:  `config : { a : 1 }; config : { b : 2 }`,
			nsName: "config",
			expected: map[string]any{
				"a": 1,
				"b": 2,
			},
		},
		{
			name:   "nested block merge",
			input:  `x : { inner : { a : 1; b : 2 } }; x : { inner : { a : 10 } }`,
			nsName: "x",
			expected: map[string]any{
				"inner": map[string]any{
					"a": 10,
					"b": 2,
				},
			},
		},
		{
			name:   "config log-level override",
			input:  `config : { log-level : "info"; log-format : "json" }; config : { log-level : "TRACE" }`,
			nsName: "config",
			expected: map[string]any{
				"log-level":  "TRACE",
				"log-format": "json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(
				context.Background(), tt.nsName, nil,
			)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			m, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map[string]any, got %T", result)
			}

			for key, want := range tt.expected {
				got, exists := m[key]
				if !exists {
					t.Errorf("missing key %q", key)

					continue
				}

				// Handle nested maps
				if wantMap, isMap := want.(map[string]any); isMap {
					gotMap, isGotMap := got.(map[string]any)
					if !isGotMap {
						t.Errorf("key %q: expected map, got %T", key, got)

						continue
					}

					for k, wv := range wantMap {
						if gv, ok := gotMap[k]; !ok {
							t.Errorf("key %q.%q: missing", key, k)
						} else if !valuesEqual(gv, wv) {
							t.Errorf("key %q.%q: expected %v, got %v",
								key, k, wv, gv)
						}
					}

					continue
				}

				if !valuesEqual(got, want) {
					t.Errorf("key %q: expected %v (%T), got %v (%T)",
						key, want, want, got, got)
				}
			}
		})
	}
}

func TestEvaluateNamespace_ExprShadowing(t *testing.T) {
	input := `x : 1; x : 2`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "x", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if !valuesEqual(result, 2) {
		t.Errorf("expected 2, got %v (%T)", result, result)
	}
}

func TestEvaluateExpr_BlockShadowing(t *testing.T) {
	input := `config : { a : 1; b : 2 }; config : { a : 3 }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(context.Background(), `config`)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}

	if !valuesEqual(m["a"], 3) {
		t.Errorf("expected a=3, got %v", m["a"])
	}

	if !valuesEqual(m["b"], 2) {
		t.Errorf("expected b=2, got %v", m["b"])
	}
}

// valuesEqual compares two values, handling int/int64 coercion.
func valuesEqual(a, b any) bool {
	// Handle int/int64 coercion
	aInt, aIsInt := toInt64(a)
	bInt, bIsInt := toInt64(b)

	if aIsInt && bIsInt {
		return aInt == bInt
	}

	return a == b
}

func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int64:
		return val, true
	default:
		return 0, false
	}
}

// TestEvaluateNamespace_LexicalScope_ForwardRefFails checks that a top-level
// namespace cannot reference another namespace defined after it (forward ref).
func TestEvaluateNamespace_LexicalScope_ForwardRefFails(t *testing.T) {
	// server references port, but port is defined AFTER server.
	input := `server : "localhost:" + string(port); port : 8080`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = a.EvaluateNamespace(context.Background(), "server", nil)
	if err == nil {
		t.Error("expected error for forward reference, got nil")
	}
}

// TestEvaluateNamespace_LexicalScope_BackwardRefWorks checks that a top-level
// namespace can reference another namespace defined before it (backward ref).
func TestEvaluateNamespace_LexicalScope_BackwardRefWorks(t *testing.T) {
	input := `port : 8080; server : "localhost:" + string(port)`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := a.EvaluateNamespace(context.Background(), "server", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "localhost:8080" {
		t.Errorf("expected %q, got %v", "localhost:8080", result)
	}
}

// TestEvaluateNamespace_LexicalScope_BlockForwardRefFails checks that a block
// entry cannot reference a sibling defined after it (block-level forward ref).
func TestEvaluateNamespace_LexicalScope_BlockForwardRefFails(t *testing.T) {
	// y references x which is defined AFTER y in the block.
	input := `config : { y : x + 1; x : 10 }`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = a.EvaluateNamespace(context.Background(), "config", nil)
	if err == nil {
		t.Error("expected error for block forward reference, got nil")
	}
}

// TestEvaluateNamespace_LexicalScope_BlockBackwardRefWorks checks that a block
// entry can reference a sibling defined before it (block-level backward ref).
func TestEvaluateNamespace_LexicalScope_BlockBackwardRefWorks(t *testing.T) {
	input := `config : { x : 10; y : x + 1 }`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := a.EvaluateNamespace(context.Background(), "config", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	if !valuesEqual(m["y"], 11) {
		t.Errorf("expected y=11, got %v", m["y"])
	}
}

// TestEvaluateNamespace_LexicalScope_OuterScopeVisibleInDescendant checks that
// a top-level namespace is accessible from within a nested block.
func TestEvaluateNamespace_LexicalScope_OuterScopeVisibleInDescendant(t *testing.T) {
	input := `level : "info"; config : { log : { default : level } }`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := a.EvaluateNamespace(context.Background(), "config", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	log, ok := m["log"].(map[string]any)
	if !ok {
		t.Fatalf("expected config.log to be map, got %T", m["log"])
	}

	if log["default"] != "info" {
		t.Errorf("expected config.log.default=%q, got %v", "info", log["default"])
	}
}

// TestEvaluateNamespace_LexicalScope_SelfRecursion checks that a parameterized
// namespace can call itself recursively (self-reference is in scope via
// inclusive visible slice).
func TestEvaluateNamespace_LexicalScope_SelfRecursion(t *testing.T) {
	input := `fib n : n <= 1 ? n : fib(n-1) + fib(n-2)`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := a.EvaluateNamespace(context.Background(), "fib", []string{"7"})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if !valuesEqual(result, 13) {
		t.Errorf("expected fib(7)=13, got %v (%T)", result, result)
	}
}

// TestEvaluateNamespace_LexicalScope_DuplicateBlockMergeUnchanged checks that
// the duplicate block+block merging behaviour is unaffected by lexical scoping.
func TestEvaluateNamespace_LexicalScope_DuplicateBlockMergeUnchanged(t *testing.T) {
	input := `config : { a : 1; b : 2 }; config : { a : 3 }`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := a.EvaluateNamespace(context.Background(), "config", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	if !valuesEqual(m["a"], 3) {
		t.Errorf("expected a=3, got %v", m["a"])
	}

	if !valuesEqual(m["b"], 2) {
		t.Errorf("expected b=2, got %v", m["b"])
	}
}

// TestEvaluateNamespace_LexicalScope_DuplicateExprShadowUnchanged checks that
// the last duplicate expression wins (unchanged behaviour).
func TestEvaluateNamespace_LexicalScope_DuplicateExprShadowUnchanged(t *testing.T) {
	input := `x : 1; x : 2`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := a.EvaluateNamespace(context.Background(), "x", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if !valuesEqual(result, 2) {
		t.Errorf("expected 2, got %v (%T)", result, result)
	}
}

// TestEvaluateExpr_LexicalScope_SeesAllNamespaces checks that EvaluateExpr
// (REPL mode) sees ALL namespaces regardless of definition order.
func TestEvaluateExpr_LexicalScope_SeesAllNamespaces(t *testing.T) {
	// server is defined before port, but EvaluateExpr has full scope.
	input := `server : "localhost:" + string(port); port : 8080`

	a, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// EvaluateExpr should succeed: it has unrestricted scope.
	result, err := a.EvaluateExpr(context.Background(), "server")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "localhost:8080" {
		t.Errorf("expected %q, got %v", "localhost:8080", result)
	}
}

func TestEvaluateNamespace_SelectiveEval_TransitiveDep(t *testing.T) {
	input := `a : 1; b : a + 1; c : b + 1; unused : 999`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "c", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	switch v := result.(type) {
	case int:
		if v != 3 {
			t.Errorf("expected 3, got %v", v)
		}
	case int64:
		if v != 3 {
			t.Errorf("expected 3, got %v", v)
		}
	default:
		t.Errorf("expected int, got %T: %v", result, result)
	}
}

func TestEvaluateNamespace_SelectiveEval_HyphenatedName(t *testing.T) {
	input := `log-level : "debug"; config : log-level`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(context.Background(), "config", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "debug" {
		t.Errorf("expected \"debug\", got %v", result)
	}
}
