package lang

import (
	"testing"
)

func TestEvaluateNamespace_Simple(t *testing.T) {
	input := `greeting : "Hello, World!"`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "greeting", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", result)
	}
}

func TestEvaluateNamespace_Number(t *testing.T) {
	input := `answer : 42`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "answer", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != int64(42) {
		t.Errorf("expected 42, got %v (%T)", result, result)
	}
}

func TestEvaluateNamespace_Boolean(t *testing.T) {
	input := `flag : true`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "flag", nil)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != true {
		t.Errorf("expected true, got %v", result)
	}
}

func TestEvaluateNamespace_Tuple(t *testing.T) {
	input := `config : { host : "localhost", port : 8080 }`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "config", nil)
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

func TestEvaluateNamespace_WithParameterSimple(t *testing.T) {
	// Bare strings are now treated as strings automatically (no quotes needed)
	input := `greet name : {{ name }}`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Pass a bare string - no quotes needed!
	result, err := ast.EvaluateNamespace(t.Context(), "greet", []string{"Alice"})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "Alice" {
		t.Errorf("expected 'Alice', got %v", result)
	}
}

func TestEvaluateNamespace_ExprArithmetic(t *testing.T) {
	// Test arithmetic with known values in the tuple
	// Note: Parameters are typed as any(nil) at compile time, so arithmetic
	// on parameters requires runtime type coercion via expr-lang functions
	input := `math : { a : 10, b : 5, sum : {{ a + b }} }`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "math", nil)
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

func TestEvaluateNamespace_EnvFunction(t *testing.T) {
	input := `home : {{ env("HOME") }}`

	processEnv := []string{"HOME=/home/testuser"}

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true), WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "home", nil, WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "/home/testuser" {
		t.Errorf("expected '/home/testuser', got %v", result)
	}
}

func TestEvaluateNamespace_NotFound(t *testing.T) {
	input := `foo : "bar"`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = ast.EvaluateNamespace(t.Context(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent definition")
	}
}

func TestEvaluateNamespace_ParamCountMismatch(t *testing.T) {
	input := `greet name : {{ name }}`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = ast.EvaluateNamespace(t.Context(), "greet", nil)
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
		{"nil", nil, "nil"},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"string_simple", "hello", `"hello"`},
		{"string_quoted", "hello world", `"hello world"`},
		{"string_special", "a:b", `"a:b"`},
		{"slice", []any{"a", "b"}, `["a", "b"]`},
		{"slice_empty", []any{}, "[]"},
		{"map", map[string]any{"x": 1}, `{"x": 1}`},
		{"map_empty", map[string]any{}, "{}"},
		{"map_hyphenated", map[string]any{"my-key": 42}, `{"my-key": 42}`},
		{"nested_slice", []any{[]any{1, 2}, []any{3, 4}}, "[[1, 2], [3, 4]]"},
		{"nested_map", map[string]any{"outer": map[string]any{"inner": "value"}}, `{"outer": {"inner": "value"}}`},
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

func TestEvaluateNamespace_NestedTuple(t *testing.T) {
	input := `server : {
		http : { host : "localhost", port : 80 },
		https : { host : "localhost", port : 443 }
	}`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "server", nil)
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

func TestEvaluateNamespace_ExprInTuple(t *testing.T) {
	input := `math : { a : 10, b : {{ a * 2 }} }`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateNamespace(t.Context(), "math", nil)
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

func TestEvaluateNamespace_TypedParameters(t *testing.T) {
	// Test that parameters can be passed as native aenv values, not just strings

	tests := []struct {
		name     string
		input    string
		defName  string
		args     []string
		expected any
	}{
		{
			name:     "bare_string_param",
			input:    `greet name : name`,
			defName:  "greet",
			args:     []string{"Alice"}, // No quotes needed!
			expected: "Alice",
		},
		{
			name:     "quoted_string_param",
			input:    `greet name : name`,
			defName:  "greet",
			args:     []string{`"Alice"`}, // Quoted strings still work
			expected: "Alice",
		},
		{
			name:     "number_param",
			input:    `double n : {{ n * 2 }}`,
			defName:  "double",
			args:     []string{"21"},
			expected: int(42),
		},
		{
			name:     "bool_param",
			input:    `negate flag : {{ !flag }}`,
			defName:  "negate",
			args:     []string{"true"},
			expected: false,
		},
		{
			name:     "identifier_param",
			input:    `base : 100; add n : {{ base + n }}`,
			defName:  "add",
			args:     []string{"42"},
			expected: int(142),
		},
		{
			name:     "identifier_reference_param",
			input:    `base : 100; getValue x : x`,
			defName:  "getValue",
			args:     []string{"base"}, // "base" is a known identifier, so it references it
			expected: int64(100),
		},
		{
			name:     "tuple_param",
			input:    `getPort config : {{ config.port }}`,
			defName:  "getPort",
			args:     []string{`{ host : "localhost", port : 8080 }`},
			expected: int64(8080),
		},
		{
			name:     "string_with_spaces",
			input:    `echo msg : msg`,
			defName:  "echo",
			args:     []string{"hello world"}, // Bare string with space
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(t.Context(), tt.input, WithCompileExprs(true))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(t.Context(), tt.defName, tt.args)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			// Handle int/int64 comparison
			switch expected := tt.expected.(type) {
			case int:
				switch got := result.(type) {
				case int:
					if got != expected {
						t.Errorf("expected %v, got %v", expected, got)
					}
				case int64:
					if got != int64(expected) {
						t.Errorf("expected %v, got %v", expected, got)
					}
				default:
					t.Errorf("expected int, got %T: %v", result, result)
				}
			default:
				if result != tt.expected {
					t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
				}
			}
		})
	}
}

func TestEvaluateNamespace_NestedDefinitionParam(t *testing.T) {
	// Test passing a nested definition as a parameter
	input := `apply f : f`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Pass a definition that evaluates to a tuple
	result, err := ast.EvaluateNamespace(t.Context(), "apply", []string{`config : { x : 1, y : 2 }`})
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T: %v", result, result)
	}

	if m["x"] != int64(1) {
		t.Errorf("expected x=1, got %v", m["x"])
	}

	if m["y"] != int64(2) {
		t.Errorf("expected y=2, got %v", m["y"])
	}
}

func TestEvaluateNamespace_DefFunctionInExpr(t *testing.T) {
	// Test calling parameterized definitions from within expressions

	tests := []struct {
		name     string
		input    string
		defName  string
		args     []string
		expected any
	}{
		{
			name:     "simple_def_call",
			input:    `double x : {{ x * 2 }}; result : {{ double(21) }}`,
			defName:  "result",
			args:     nil,
			expected: int(42),
		},
		{
			name:     "def_call_with_string",
			input:    `greet name : {{ "Hello, " + name }}; message : {{ greet("World") }}`,
			defName:  "message",
			args:     nil,
			expected: "Hello, World",
		},
		{
			name:     "nested_def_call",
			input:    `inc x : {{ x + 1 }}; twice x : {{ inc(inc(x)) }}; result : {{ twice(5) }}`,
			defName:  "result",
			args:     nil,
			expected: int(7),
		},
		{
			name:     "def_call_with_param",
			input:    `add a b : {{ a + b }}; compute x : {{ add(x, 10) }}`,
			defName:  "compute",
			args:     []string{"5"},
			expected: int(15),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(t.Context(), tt.input, WithCompileExprs(true))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(t.Context(), tt.defName, tt.args)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)",
					tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType Type
	}{
		{"string", `"hello"`, TypeString},
		{"string_With_escapes", `"line1\nline2\tindent\"quote\""`, TypeString},
		{"number_int", "42", TypeNumber},
		{"number_float", "3.14", TypeNumber},
		{"number_negative", "-5", TypeNumber},
		{"number_float_negative", "-2.71", TypeNumber},
		{"number_float_no_leading_zero", ".5", TypeNumber},
		{"number_float_negative_no_leading_zero", "-.25", TypeNumber},
		{"number_float_exponent", "1e3", TypeNumber},
		{"number_float_exponent_negative", "2.5e-4", TypeNumber},
		{"number_float_exponent_no_leading_zero", ".1e2", TypeNumber},
		{"number_float_exponent_negative_no_leading_zero", "-.1e-2", TypeNumber},
		{"number_float_exponent_uppercase", "1E3", TypeNumber},
		{"number_float_exponent_negative_uppercase", "2.5E-4", TypeNumber},
		{"number_base2", "0b1010", TypeNumber},
		{"number_base8", "0o17", TypeNumber},
		{"number_base16", "0x1A", TypeNumber},
		{"bool_true", "true", TypeBoolean},
		{"bool_false", "false", TypeBoolean},
		{"identifier", "foo", TypeIdentifier},
		{"tuple", "{ a : 1, b : 2 }", TypeTuple},
		{"definition", "x : 42", TypeNamespace},
		{"expr", "{{ 1 + 2 }}", TypeExpr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := ParseValue(t.Context(), tt.input)
			if err != nil {
				t.Fatalf("ParseValue error: %v", err)
			}

			if val.Type != tt.wantType {
				t.Errorf("expected type %v, got %v", tt.wantType, val.Type)
			}
		})
	}
}

func TestEvaluateExpr_SimpleNamespace(t *testing.T) {
	input := `greeting : "Hello, World!"`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "greeting")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", result)
	}
}

func TestEvaluateExpr_Arithmetic(t *testing.T) {
	input := `a : 10; b : 5`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "a + b")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	switch v := result.(type) {
	case int:
		if v != 15 {
			t.Errorf("expected 15, got %v", v)
		}
	case int64:
		if v != 15 {
			t.Errorf("expected 15, got %v", v)
		}
	default:
		t.Errorf("expected int, got %T: %v", result, result)
	}
}

func TestEvaluateExpr_FunctionCall(t *testing.T) {
	input := `double x : {{ x * 2 }}`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), `double(21)`)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

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

func TestEvaluateExpr_DotAccess(t *testing.T) {
	input := `config : { host : "localhost", port : 8080 }`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "config.host")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "localhost" {
		t.Errorf("expected 'localhost', got %v", result)
	}
}

func TestEvaluateExpr_EnvFunction(t *testing.T) {
	input := `x : 1`
	processEnv := []string{"FOO=bar"}

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true), WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), `env("FOO")`, WithProcessEnv(processEnv))
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "bar" {
		t.Errorf("expected 'bar', got %v", result)
	}
}

func TestEvaluateExpr_CompileError(t *testing.T) {
	input := `x : 1`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, err = ast.EvaluateExpr(t.Context(), "+++invalid")
	if err == nil {
		t.Error("expected compile error")
	}
}

func TestEvaluateExpr_HyphenatedMemberAccess(t *testing.T) {
	input := `config : { log-pretty : true, http-port : 8080 }`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "config.log-pretty")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != true {
		t.Errorf("expected true, got %v", result)
	}

	result, err = ast.EvaluateExpr(t.Context(), "config.http-port")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != int64(8080) {
		t.Errorf("expected 8080, got %v (%T)", result, result)
	}
}

func TestEvaluateExpr_HyphenatedTopLevel(t *testing.T) {
	input := `log-pretty : true`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "log-pretty")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != true {
		t.Errorf("expected true, got %v", result)
	}
}

func TestEvaluateExpr_HyphenatedPreservesSubtraction(t *testing.T) {
	input := `a : 10; b : 3`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "a - b")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != 7 && result != int64(7) {
		t.Errorf("expected 7, got %v (%T)", result, result)
	}
}

func TestEvaluateExpr_HyphenatedMultiSegment(t *testing.T) {
	input := `config : { log-pretty-format : "json" }`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "config.log-pretty-format")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	if result != "json" {
		t.Errorf("expected \"json\", got %v", result)
	}
}

func TestResolveForEnv_ExprResolution(t *testing.T) {
	// Test that non-cyclic expr-valued namespaces are resolved in the
	// runtime environment of other expressions.

	tests := []struct {
		name     string
		input    string
		defName  string
		args     []string
		expected any
	}{
		{
			name:     "expr_ref_non_cyclic",
			input:    `a : {{ 10 + 5 }}; b : {{ a * 2 }}`,
			defName:  "b",
			args:     nil,
			expected: int(30),
		},
		{
			name:     "expr_ref_chain",
			input:    `x : {{ 3 }}; y : {{ x + 1 }}; z : {{ y + 1 }}`,
			defName:  "z",
			args:     nil,
			expected: int(5),
		},
		{
			name:     "expr_ref_with_literal",
			input:    `base : 100; computed : {{ base + 1 }}; result : {{ computed * 2 }}`,
			defName:  "result",
			args:     nil,
			expected: int(202),
		},
		{
			name:     "expr_ref_string_concat",
			input:    `prefix : {{ "Hello" }}; greeting : {{ prefix + ", World!" }}`,
			defName:  "greeting",
			args:     nil,
			expected: "Hello, World!",
		},
		{
			name:     "expr_ref_in_tuple",
			input:    `factor : {{ 10 }}; config : { scale : {{ factor * 3 }} }`,
			defName:  "config",
			args:     nil,
			expected: map[string]any{"scale": int(30)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(t.Context(), tt.input, WithCompileExprs(true))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			result, err := ast.EvaluateNamespace(t.Context(), tt.defName, tt.args)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}

			switch expected := tt.expected.(type) {
			case int:
				switch got := result.(type) {
				case int:
					if got != expected {
						t.Errorf("expected %v, got %v", expected, got)
					}
				case int64:
					if got != int64(expected) {
						t.Errorf("expected %v, got %v", expected, got)
					}
				default:
					t.Errorf("expected int, got %T: %v", result, result)
				}
			case map[string]any:
				got, ok := result.(map[string]any)
				if !ok {
					t.Fatalf("expected map, got %T: %v", result, result)
				}

				for k, v := range expected {
					gotVal, exists := got[k]
					if !exists {
						t.Errorf("expected key %q in result", k)

						continue
					}

					switch ev := v.(type) {
					case int:
						switch gv := gotVal.(type) {
						case int:
							if gv != ev {
								t.Errorf("key %q: expected %v, got %v", k, ev, gv)
							}
						case int64:
							if gv != int64(ev) {
								t.Errorf("key %q: expected %v, got %v", k, ev, gv)
							}
						default:
							t.Errorf("key %q: expected int, got %T: %v", k, gotVal, gotVal)
						}
					default:
						if gotVal != v {
							t.Errorf("key %q: expected %v, got %v", k, v, gotVal)
						}
					}
				}
			default:
				if result != tt.expected {
					t.Errorf("expected %v (%T), got %v (%T)",
						tt.expected, tt.expected, result, result)
				}
			}
		})
	}
}

func TestResolveForEnv_CycleDetection(t *testing.T) {
	// Test that mutually-referencing expr namespaces don't cause infinite
	// recursion. The cycle should be broken by returning nil for the
	// back-edge.

	tests := []struct {
		name    string
		input   string
		defName string
	}{
		{
			name:    "mutual_cycle",
			input:   `a : {{ b }}; b : {{ a }}`,
			defName: "a",
		},
		{
			name:    "self_referencing",
			input:   `x : {{ x + 1 }}`,
			defName: "x",
		},
		{
			name:    "indirect_cycle",
			input:   `a : {{ b + 1 }}; b : {{ c + 1 }}; c : {{ a + 1 }}`,
			defName: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(t.Context(), tt.input, WithCompileExprs(true))
			if err != nil {
				// Compile-time error is an acceptable outcome for cycles:
				// the type checker may reject the expression before runtime
				// cycle detection can engage.
				t.Logf("compile-time error (acceptable): %v", err)

				return
			}

			_, err = ast.EvaluateNamespace(t.Context(), tt.defName, nil)
			if err != nil {
				t.Logf("runtime error (acceptable): %v", err)
			} else {
				// Cyclic expr resolution produces nil for the back-edge,
				// which may propagate as a result. This is acceptable
				// as long as we don't hang or stack overflow.
				t.Log("cycle resolved without error (nil propagated)")
			}
			// The key assertion: we reached this point without hanging
			// or panicking due to infinite recursion.
		})
	}
}

func TestEvaluateExpr_CrossNamespaceExpr(t *testing.T) {
	// Test that EvaluateExpr can reference expr-valued namespaces.
	input := `base : {{ 21 }}`

	ast, err := ParseString(t.Context(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ast.EvaluateExpr(t.Context(), "base * 2")
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}

	switch v := result.(type) {
	case int, int64:
		if v != 42 {
			t.Errorf("expected 42, got %v", v)
		}
	default:
		t.Errorf("expected int, got %T: %v", result, result)
	}
}
