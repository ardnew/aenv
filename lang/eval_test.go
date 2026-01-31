package lang

import (
	"testing"
)

func TestEvaluateDefinition_Simple(t *testing.T) {
	input := `greeting : "Hello, World!"`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("greeting", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", result)
	}
}

func TestEvaluateDefinition_Number(t *testing.T) {
	input := `answer : 42`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("answer", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != int64(42) {
		t.Errorf("expected 42, got %v (%T)", result, result)
	}
}

func TestEvaluateDefinition_Boolean(t *testing.T) {
	input := `flag : true`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("flag", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != true {
		t.Errorf("expected true, got %v", result)
	}
}

func TestEvaluateDefinition_Tuple(t *testing.T) {
	input := `config : { host : "localhost", port : 8080 }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("config", nil)
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

	if m["port"] != int64(8080) {
		t.Errorf("expected port=8080, got %v (%T)", m["port"], m["port"])
	}
}

func TestEvaluateDefinition_WithParameter(t *testing.T) {
	// For parameters, expr-lang needs to know the type at compile time
	// Since parameters are typed as any(nil), we can concatenate strings
	// using the string() function
	input := `greet name : {{ name }}`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("greet", []string{"Alice"})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "Alice" {
		t.Errorf("expected 'Alice', got %v", result)
	}
}

func TestEvaluateDefinition_ExprArithmetic(t *testing.T) {
	// Test arithmetic with known values in the tuple
	// Note: Parameters are typed as any(nil) at compile time, so arithmetic
	// on parameters requires runtime type coercion via expr-lang functions
	input := `math : { a : 10, b : 5, sum : {{ a + b }} }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("math", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	// a + b = 15
	switch sum := m["sum"].(type) {
	case int:
		if sum != 15 {
			t.Errorf("expected sum=15, got %v", sum)
		}
	case int64:
		if sum != 15 {
			t.Errorf("expected sum=15, got %v", sum)
		}
	default:
		t.Errorf("expected sum to be int or int64, got %T", m["sum"])
	}
}

func TestEvaluateDefinition_EnvFunction(t *testing.T) {
	input := `home : {{ env("HOME") }}`

	processEnv := []string{"HOME=/home/testuser"}

	ast, err := ParseString(input, WithCompileExprs(true), WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("home", nil, WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "/home/testuser" {
		t.Errorf("expected '/home/testuser', got %v", result)
	}
}

func TestEvaluateDefinition_NotFound(t *testing.T) {
	input := `foo : "bar"`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = ast.EvaluateDefinition("nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent definition")
	}
}

func TestEvaluateDefinition_ParamCountMismatch(t *testing.T) {
	input := `greet name : {{ name }}`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = ast.EvaluateDefinition("greet", nil)
	if err == nil {
		t.Error("expected error for parameter count mismatch")
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect string
	}{
		{"nil", nil, ""},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"string_simple", "hello", "hello"},
		{"string_quoted", "hello world", `"hello world"`},
		{"string_special", "a:b", `"a:b"`},
		{"slice", []any{"a", "b"}, "{ a, b }"},
		{"map", map[string]any{"x": 1}, "{ x : 1 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatResult(tt.input)
			if result != tt.expect {
				t.Errorf("FormatResult(%v) = %q, want %q", tt.input, result, tt.expect)
			}
		})
	}
}

func TestEvaluateDefinition_NestedTuple(t *testing.T) {
	input := `server : {
		http : { host : "localhost", port : 80 },
		https : { host : "localhost", port : 443 }
	}`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("server", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	http, ok := m["http"].(map[string]any)
	if !ok {
		t.Fatalf("expected http to be map, got %T", m["http"])
	}

	if http["port"] != int64(80) {
		t.Errorf("expected http.port=80, got %v", http["port"])
	}
}

func TestEvaluateDefinition_ExprInTuple(t *testing.T) {
	input := `math : { a : 10, b : {{ a * 2 }} }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateDefinition("math", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	if m["a"] != int64(10) {
		t.Errorf("expected a=10, got %v", m["a"])
	}

	switch b := m["b"].(type) {
	case int:
		if b != 20 {
			t.Errorf("expected b=20, got %v", b)
		}
	case int64:
		if b != 20 {
			t.Errorf("expected b=20, got %v", b)
		}
	default:
		t.Errorf("expected b to be int or int64, got %T", m["b"])
	}
}
