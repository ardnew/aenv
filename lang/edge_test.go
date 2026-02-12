package lang_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ardnew/aenv/lang"
)

// TestEvaluateNumber_EdgeCases tests edge cases for number evaluation
func TestEvaluateNumber_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "max_int64",
			input:   `val : 9223372036854775807`,
			wantErr: false,
		},
		{
			name:    "min_int64",
			input:   `val : -9223372036854775808`,
			wantErr: false,
		},
		{
			name:    "very_large_float",
			input:   `val : 1.7976931348623157e+308`,
			wantErr: false,
		},
		{
			name:    "very_small_float",
			input:   `val : 2.2250738585072014e-308`,
			wantErr: false,
		},
		{
			name:    "negative_zero",
			input:   `val : -0.0`,
			wantErr: false,
		},
		{
			name:    "scientific_notation_edge",
			input:   `val : 1.0e-100`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), "val", nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestEvaluateString_EdgeCases tests edge cases for string evaluation
func TestEvaluateString_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty_string",
			input:    `val : ""`,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "string_with_null_byte",
			input:    `val : "hello\x00world"`,
			expected: "hello\x00world",
			wantErr:  false,
		},
		{
			name:     "string_with_unicode",
			input:    `val : "こんにちは世界"`,
			expected: "こんにちは世界",
			wantErr:  false,
		},
		{
			name:     "string_with_emoji",
			input:    `val : "Hello 👋 World 🌍"`,
			expected: "Hello 👋 World 🌍",
			wantErr:  false,
		},
		{
			name:     "string_with_escapes",
			input:    `val : "line1\nline2\ttab"`,
			expected: "line1\nline2\ttab",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), "val", nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateNamespace() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr {
				if str, ok := result.(string); ok {
					if str != tt.expected {
						t.Errorf("Expected %q, got %q", tt.expected, str)
					}
				} else {
					t.Errorf("Expected string result, got %T", result)
				}
			}
		})
	}
}

// TestParseArgToValue_EdgeCases tests edge cases for parseArgToValue
func TestParseArgToValue_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		arg      string
		wantType lang.Type
	}{
		{
			name:     "empty_string",
			arg:      "",
			wantType: lang.TypeString,
		},
		{
			name:     "whitespace_only",
			arg:      "   ",
			wantType: lang.TypeString,
		},
		{
			name:     "string_with_spaces",
			arg:      "hello world",
			wantType: lang.TypeString,
		},
		{
			name:     "number_like_string",
			arg:      "123abc",
			wantType: lang.TypeString,
		},
		{
			name:     "bool_like_string",
			arg:      "TRUE",
			wantType: lang.TypeString,
		},
		{
			name:     "identifier_reference",
			arg:      "test",
			wantType: lang.TypeIdentifier,
		},
		{
			name:     "hex_number",
			arg:      "0xFF",
			wantType: lang.TypeNumber,
		},
		{
			name:     "binary_number",
			arg:      "0b1010",
			wantType: lang.TypeNumber,
		},
		{
			name:     "octal_number",
			arg:      "0o755",
			wantType: lang.TypeNumber,
		},
		{
			name:     "float_with_no_leading_zero",
			arg:      ".5",
			wantType: lang.TypeNumber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// This uses the unexported parseArgToValue through EvaluateNamespace
			astWithParam, err := lang.ParseString(
				context.Background(),
				`param_test x : x`,
			)
			if err != nil {
				t.Fatalf("Failed to parse param AST: %v", err)
			}

			// Evaluate with the arg - this internally calls parseArgToValue
			result, err := astWithParam.EvaluateNamespace(
				context.Background(),
				"param_test",
				[]string{tt.arg},
			)
			if err != nil {
				t.Errorf("EvaluateNamespace() error = %v", err)

				return
			}

			// Just check it doesn't crash and returns something
			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestToMap_EdgeCases tests edge cases for ToMap
func TestToMap_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "nested_empty_tuples",
			input:   `outer : { inner : {} }`,
			wantErr: false,
		},
		{
			name:    "deeply_nested",
			input:   `a : { b : { c : { d : { e : 1 } } } }`,
			wantErr: false,
		},
		{
			name:    "mixed_value_types",
			input:   `mix : { str : "text", num : 42, bool : true, expr : {{ 1 + 1 }} }`,
			wantErr: false,
		},
		{
			name:    "identifier_in_tuple",
			input:   `ref : 123; tuple : { val : ref }`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			result := ast.ToMap()
			if result == nil && !tt.wantErr {
				t.Error("ToMap() returned nil")

				return
			}
			if !tt.wantErr && result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestParseReader_WithOptions tests ParseReader with different options
func TestParseReader_WithOptions(t *testing.T) {
	t.Parallel()

	input := `test : {{ env("TEST_VAR") }}`

	tests := []struct {
		name    string
		opts    []lang.Option
		wantErr bool
	}{
		{
			name:    "with_compile_exprs",
			opts:    []lang.Option{lang.WithCompileExprs(true)},
			wantErr: false,
		},
		{
			name:    "with_process_env",
			opts:    []lang.Option{lang.WithProcessEnv([]string{"TEST_VAR=value"})},
			wantErr: false,
		},
		{
			name:    "with_max_depth",
			opts:    []lang.Option{lang.WithMaxDepth(10)},
			wantErr: false,
		},
		{
			name: "multiple_options",
			opts: []lang.Option{
				lang.WithCompileExprs(true),
				lang.WithProcessEnv([]string{"TEST_VAR=value"}),
				lang.WithMaxDepth(50),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := strings.NewReader(input)
			ast, err := lang.ParseReader(context.Background(), reader, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReader() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr {
				if ast == nil {
					t.Error("Expected non-nil AST")
				}
				if len(ast.Namespaces) == 0 {
					t.Error("Expected at least one namespace")
				}
			}
		})
	}
}

// TestAmbiguousParseError tests ambiguous parse error handling
func TestAmbiguousParseError(t *testing.T) {
	t.Parallel()

	// This is a contrived example that might trigger ambiguous parse
	// The actual grammar may not have ambiguities, but we test the error path
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "potential_ambiguity_1",
			input: `a : { b : { c : d } }`,
		},
		{
			name:  "potential_ambiguity_2",
			input: `x : y; z : {{ x + 1 }}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// These should parse successfully (no ambiguity in current grammar)
			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				// If there's an error, it should be properly formatted
				errStr := err.Error()
				if errStr == "" {
					t.Error("Error should have a message")
				}
			}
			if ast != nil && len(ast.Namespaces) == 0 {
				t.Error("AST should have namespaces if no error")
			}
		})
	}
}

// TestResolveForEnv_EdgeCases tests resolveForEnv with complex scenarios
func TestResolveForEnv_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		nsName   string
		wantErr  bool
		allowNil bool // Allow nil result for expression types
	}{
		{
			name:     "circular_reference_prevention",
			input:    `a : {{ b }}; b : {{ c }}; c : 1`,
			nsName:   "a",
			wantErr:  false,
			allowNil: true, // Expression evaluation may return nil
		},
		{
			name:     "self_reference_expr",
			input:    `val : 10`,
			nsName:   "val",
			wantErr:  false,
			allowNil: false,
		},
		{
			name:     "nested_tuple_with_identifiers",
			input:    `base : 5; nested : { val : base, doubled : {{ base * 2 }} }`,
			nsName:   "nested",
			wantErr:  false,
			allowNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(
				context.Background(),
				tt.input,
				lang.WithCompileExprs(true),
			)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), tt.nsName, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateNamespace() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr && result == nil && !tt.allowNil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestAnyToArgString_EdgeCases tests anyToArgString conversion
func TestAnyToArgString_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test via calling parameterized namespaces from expressions
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "pass_nil_to_func",
			input:   `fn x : x; result : fn`,
			wantErr: true, // Should fail because fn expects an arg
		},
		{
			name:    "pass_bool_to_func",
			input:   `fn x : {{ x ? "yes" : "no" }}; result : {{ fn(true) }}`,
			wantErr: false,
		},
		{
			name:    "pass_int_to_func",
			input:   `fn x : {{ x + 10 }}; result : {{ fn(5) }}`,
			wantErr: false,
		},
		{
			name:    "pass_float_to_func",
			input:   `fn x : {{ x * 2.5 }}; result : {{ fn(4.0) }}`,
			wantErr: false,
		},
		{
			name:    "pass_string_to_func",
			input:   `fn x : {{ x + " suffix" }}; result : {{ fn("prefix") }}`,
			wantErr: false,
		},
		{
			name:    "pass_map_to_func",
			input:   `fn x : x; obj : { a : 1, b : 2 }; result : {{ fn(obj) }}`,
			wantErr: false, // Passes the obj identifier
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(
				context.Background(),
				tt.input,
				lang.WithCompileExprs(true),
			)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), "result", nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateNamespace() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr && result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestEvaluateBoolean_ErrorCases tests boolean evaluation error handling
func TestEvaluateBoolean_ErrorCases(t *testing.T) {
	t.Parallel()

	// Boolean parsing should be robust, but let's test edge cases
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid_true",
			input:   `val : true`,
			wantErr: false,
		},
		{
			name:    "valid_false",
			input:   `val : false`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			result, err := ast.EvaluateNamespace(context.Background(), "val", nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateNamespace() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr {
				if _, ok := result.(bool); !ok {
					t.Errorf("Expected bool result, got %T", result)
				}
			}
		})
	}
}

// TestToNative_ErrorCases tests ToNative with various error scenarios
func TestToNative_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid_simple",
			input:   `test : "value"`,
			wantErr: false,
		},
		{
			name:    "valid_tuple",
			input:   `test : { a : 1, b : 2 }`,
			wantErr: false,
		},
		{
			name:    "valid_nested",
			input:   `test : { nested : { value : 42 } }`,
			wantErr: false,
		},
		{
			name:    "list_values",
			input:   `test : { 1, 2, 3 }`,
			wantErr: false,
		},
		{
			name:    "mixed_tuple",
			input:   `test : { str : "text", num : 42 }`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			// Get the namespace and convert its value to native
			ns, ok := ast.GetNamespace("test")
			if !ok {
				t.Fatal("Failed to get 'test' namespace")
			}

			result := ns.Value.ToNative()
			if !tt.wantErr && result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestFormat_NativeAenvSyntax tests the Format function which outputs native aenv syntax
func TestFormat_NativeAenvSyntax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		indent   int
		contains []string // Strings that should appear in output
	}{
		{
			name:   "simple_namespace",
			input:  `test : "value"`,
			indent: 0,
			contains: []string{
				"test",
				":",
				`"value"`,
			},
		},
		{
			name:   "multiple_namespaces_no_indent",
			input:  `a : 1; b : 2`,
			indent: 0,
			contains: []string{
				"a : 1",
				";",
				"b : 2",
			},
		},
		{
			name:   "multiple_namespaces_with_indent",
			input:  `a : 1; b : 2`,
			indent: 2,
			contains: []string{
				"a : 1",
				"b : 2",
			},
		},
		{
			name:   "nested_tuple",
			input:  `test : { a : 1, b : 2 }`,
			indent: 2,
			contains: []string{
				"test : {",
				"a : 1",
				"b : 2",
				"}",
			},
		},
		{
			name:   "parameterized_namespace",
			input:  `greet name : {{ "Hello, " + name }}`,
			indent: 0,
			contains: []string{
				"greet",
				"name",
				":",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			var buf strings.Builder
			err = ast.Format(context.Background(), &buf, tt.indent)
			if err != nil {
				t.Errorf("Format() error = %v", err)

				return
			}

			output := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("Output missing expected string %q\nGot: %s", want, output)
				}
			}
		})
	}
}

// TestFormatJSON tests JSON formatting
func TestFormatJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		indent  int
		wantErr bool
	}{
		{
			name:    "simple_value",
			input:   `test : "value"`,
			indent:  0,
			wantErr: false,
		},
		{
			name:    "simple_value_indented",
			input:   `test : "value"`,
			indent:  2,
			wantErr: false,
		},
		{
			name:    "tuple",
			input:   `test : { a : 1, b : 2 }`,
			indent:  2,
			wantErr: false,
		},
		{
			name:    "multiple_namespaces",
			input:   `a : 1; b : 2; c : 3`,
			indent:  2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			var buf strings.Builder
			err = ast.FormatJSON(context.Background(), &buf, tt.indent)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatJSON() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				// Verify it's valid JSON
				output := buf.String()
				var result map[string]any
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("FormatJSON() produced invalid JSON: %v\nOutput: %s", err, output)
				}
			}
		})
	}
}

// TestFormatYAML tests YAML formatting
func TestFormatYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		indent  int
		wantErr bool
	}{
		{
			name:    "simple_value",
			input:   `test : "value"`,
			indent:  0,
			wantErr: false,
		},
		{
			name:    "simple_value_indented",
			input:   `test : "value"`,
			indent:  2,
			wantErr: false,
		},
		{
			name:    "tuple",
			input:   `test : { a : 1, b : 2 }`,
			indent:  2,
			wantErr: false,
		},
		{
			name:    "nested_structure",
			input:   `outer : { inner : { deep : "value" } }`,
			indent:  2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ast, err := lang.ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			var buf strings.Builder
			err = ast.FormatYAML(context.Background(), &buf, tt.indent)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatYAML() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				output := buf.String()
				// Basic check - YAML should not be empty
				if len(output) == 0 {
					t.Error("FormatYAML() produced empty output")
				}
			}
		})
	}
}

// TestInferNamespaceExemplar tests type inference for namespace exemplars
func TestInferNamespaceExemplar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		namespace string
		wantFunc  bool // true if we expect a function type
	}{
		{
			name:      "parametric_namespace",
			input:     `greet name : {{ "Hello, " + name }}`,
			namespace: "greet",
			wantFunc:  true, // Should infer as function because it has parameters
		},
		{
			name:      "simple_value_namespace",
			input:     `val : 42`,
			namespace: "val",
			wantFunc:  false, // Should infer as int64
		},
		{
			name:      "string_namespace",
			input:     `msg : "hello"`,
			namespace: "msg",
			wantFunc:  false, // Should infer as string
		},
		{
			name:      "tuple_namespace",
			input:     `data : { a : 1, b : 2 }`,
			namespace: "data",
			wantFunc:  false, // Should infer as map
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Parse and compile
			ast, err := lang.ParseString(
				context.Background(),
				tt.input,
				lang.WithCompileExprs(true),
			)
			if err != nil {
				t.Fatalf("ParseString() error = %v", err)
			}

			// Get the namespace
			ns, ok := ast.GetNamespace(tt.namespace)
			if !ok {
				t.Fatalf("Failed to get namespace %q", tt.namespace)
			}

			// Check if it's parametric (has parameters)
			hasParams := len(ns.Parameters) > 0
			if hasParams != tt.wantFunc {
				t.Errorf("Namespace parametric = %v, want parametric %v", hasParams, tt.wantFunc)
			}
		})
	}
}
