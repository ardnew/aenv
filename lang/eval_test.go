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
			input:  `dir : cwd()`,
			nsName: "dir",
			check: func(t *testing.T, result any) {
				if _, ok := result.(string); !ok {
					t.Errorf("expected string, got %T", result)
				}
			},
		},
		{
			name:   "platform",
			input:  `plat : platform.OS`,
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

	// Too many arguments
	_, err = ast.EvaluateNamespace(context.Background(), "greet", []string{"Alice", "Bob"})
	if err == nil {
		t.Error("expected error for too many parameters, got nil")
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

func TestEvaluateExpr_FunctionIdentifierReturnsNil(t *testing.T) {
	input := `add x y : x + y`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Evaluating the function identifier directly should return nil
	result, err := ast.EvaluateExpr(context.Background(), "add")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil for function identifier, got %v (%T)", result, result)
	}
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
