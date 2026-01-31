package lang

import (
	"testing"
)

func TestCompileExprs_NeighborReference(t *testing.T) {
	input := `test : { x : 10, y : {{ x + 1 }} }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	if def.Value.Type != TypeTuple {
		t.Fatalf("expected tuple, got %s", def.Value.Type)
	}

	// y is the second definition in the tuple
	yDef := def.Value.Tuple.Values[1]
	if yDef.Type != TypeDefinition {
		t.Fatalf("expected definition, got %s", yDef.Type)
	}

	yVal := yDef.Definition.Value
	if yVal.Type != TypeExpr {
		t.Fatalf("expected expr, got %s", yVal.Type)
	}

	if yVal.Program == nil {
		t.Fatal("expected compiled program, got nil")
	}
}

func TestCompileExprs_AncestorReference(t *testing.T) {
	input := `root : { x : 10, inner : { y : {{ x }} } }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	inner := def.Value.Tuple.Values[1].Definition
	yDef := inner.Value.Tuple.Values[0].Definition
	yVal := yDef.Value

	if yVal.Type != TypeExpr {
		t.Fatalf("expected expr, got %s", yVal.Type)
	}

	if yVal.Program == nil {
		t.Fatal("expected compiled program, got nil")
	}
}

func TestCompileExprs_ParameterInEnv(t *testing.T) {
	input := `test region : { host : {{ region }} }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	hostDef := def.Value.Tuple.Values[0].Definition
	hostVal := hostDef.Value

	if hostVal.Type != TypeExpr {
		t.Fatalf("expected expr, got %s", hostVal.Type)
	}

	if hostVal.Program == nil {
		t.Fatal("expected compiled program, got nil")
	}
}

func TestCompileExprs_EnvFunction(t *testing.T) {
	input := `test : { home : {{ env("HOME") }} }`

	ast, err := ParseString(input, WithCompileExprs(true), WithProcessEnv([]string{"HOME=/home/testuser"}))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	homeDef := def.Value.Tuple.Values[0].Definition
	homeVal := homeDef.Value

	if homeVal.Type != TypeExpr {
		t.Fatalf("expected expr, got %s", homeVal.Type)
	}

	if homeVal.Program == nil {
		t.Fatal("expected compiled program, got nil")
	}
}

func TestCompileExprs_CompileError(t *testing.T) {
	input := `test : { x : {{ +++ invalid }} }`

	_, err := ParseString(input, WithCompileExprs(true))
	if err == nil {
		t.Fatal("expected compile error, got nil")
	}
}

func TestCompileExprs_Disabled(t *testing.T) {
	input := `test : { x : {{ 1 + 2 }} }`

	// Default options do not compile expressions
	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	xDef := def.Value.Tuple.Values[0].Definition
	xVal := xDef.Value

	if xVal.Type != TypeExpr {
		t.Fatalf("expected expr, got %s", xVal.Type)
	}

	if xVal.Program != nil {
		t.Fatal("expected nil program when compilation disabled")
	}
}

func TestCompileExprs_EmptyExpr(t *testing.T) {
	input := `test : { x : {{}} }`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	xDef := def.Value.Tuple.Values[0].Definition
	xVal := xDef.Value

	if xVal.Type != TypeExpr {
		t.Fatalf("expected expr, got %s", xVal.Type)
	}

	// Empty expressions are skipped (expr-lang cannot compile "")
	if xVal.Program != nil {
		t.Error("expected nil program for empty expression")
	}
}

func TestCompileExprs_MultipleExprs(t *testing.T) {
	input := `test : {
		a : 10,
		b : {{ a + 1 }},
		c : "hello",
		d : {{ a + 2 }},
	}`

	ast, err := ParseString(input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	values := def.Value.Tuple.Values

	// b (index 1) should have compiled program
	bVal := values[1].Definition.Value
	if bVal.Program == nil {
		t.Error("expected compiled program for 'b'")
	}

	// d (index 3) should have compiled program
	dVal := values[3].Definition.Value
	if dVal.Program == nil {
		t.Error("expected compiled program for 'd'")
	}

	// a (index 0) and c (index 2) should NOT have programs
	aVal := values[0].Definition.Value
	if aVal.Program != nil {
		t.Error("expected nil program for number 'a'")
	}

	cVal := values[2].Definition.Value
	if cVal.Program != nil {
		t.Error("expected nil program for string 'c'")
	}
}

func TestCompileExprs_ExprSource(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple expression",
			input: `{{ x + 1 }}`,
			want:  "x + 1",
		},
		{
			name:  "no spaces",
			input: `{{x+1}}`,
			want:  "x+1",
		},
		{
			name:  "empty",
			input: `{{}}`,
			want:  "",
		},
		{
			name:  "with whitespace",
			input: `{{  foo  }}`,
			want:  "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewExpr(tt.input)

			got := v.ExprSource()
			if got != tt.want {
				t.Errorf("ExprSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompileExprs_ProcessEnvDefault(t *testing.T) {
	// When ProcessEnv is nil, os.Environ() is used
	env := buildProcessEnvMap(nil)
	if len(env) == 0 {
		t.Skip("no environment variables available")
	}

	// PATH should exist in most environments
	if _, ok := env["PATH"]; !ok {
		t.Skip("PATH not in environment")
	}
}

func TestCompileExprs_ProcessEnvExplicit(t *testing.T) {
	env := buildProcessEnvMap([]string{
		"FOO=bar",
		"BAZ=qux",
		"EMPTY=",
	})

	if env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got %q", env["FOO"])
	}

	if env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got %q", env["BAZ"])
	}

	if env["EMPTY"] != "" {
		t.Errorf("expected EMPTY='', got %q", env["EMPTY"])
	}
}

func TestInferTypeExemplar(t *testing.T) {
	tests := []struct {
		name string
		val  *Value
		want string // type name for comparison
	}{
		{"nil value", nil, "<nil>"},
		{"boolean", NewBool(true), "bool"},
		{"integer", NewNumber("42"), "int64"},
		{"float", NewNumber("3.14"), "float64"},
		{"scientific", NewNumber("1e10"), "float64"},
		{"string", NewString("hello"), "string"},
		{"identifier", NewIdentifier("foo"), "<nil>"},
		{"expr", NewExpr("{{x}}"), "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTypeExemplar(tt.val)

			switch tt.want {
			case "<nil>":
				if got != nil {
					t.Errorf("expected nil, got %T(%v)", got, got)
				}
			case "bool":
				if _, ok := got.(bool); !ok {
					t.Errorf("expected bool, got %T", got)
				}
			case "int64":
				if _, ok := got.(int64); !ok {
					t.Errorf("expected int64, got %T", got)
				}
			case "float64":
				if _, ok := got.(float64); !ok {
					t.Errorf("expected float64, got %T", got)
				}
			case "string":
				if _, ok := got.(string); !ok {
					t.Errorf("expected string, got %T", got)
				}
			}
		})
	}
}
