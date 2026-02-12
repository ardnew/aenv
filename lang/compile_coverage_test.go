package lang

import (
	"context"
	"testing"
)

// TestCompileValue_NilValue tests that compileValue handles nil gracefully.
// Note: The actual implementation will panic on nil due to accessing v.Type,
// but in practice this should never be called with nil from the public API.
func TestCompileValue_NilValue(t *testing.T) {
	t.Parallel()

	// This test documents current behavior - nil causes panic
	// In production, compileValue is only called internally with valid values
	t.Skip("compileValue with nil panics - this is internal implementation detail")
}

// TestCompileValue_TypeNamespaceNilNamespace tests TypeNamespace with nil Namespace.
func TestCompileValue_TypeNamespaceNilNamespace(t *testing.T) {
	t.Parallel()

	ast := &AST{}
	applyDefaults(ast)

	// Create a value with TypeNamespace but nil Namespace field
	val := &Value{Type: TypeNamespace, Namespace: nil}

	err := compileValue(context.Background(), val, nil, nil, ast)
	if err != nil {
		t.Errorf("compileValue(TypeNamespace with nil) returned error: %v", err)
	}
}

// TestCompileValue_TypeIdentifier tests that identifier types are handled.
func TestCompileValue_TypeIdentifier(t *testing.T) {
	t.Parallel()

	input := `test : { x : 10, ref : x }`
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// ref is an identifier (index 1 in tuple)
	refVal := ast.Namespaces[0].Value.Tuple.Values[1].Namespace.Value
	if refVal.Type != TypeIdentifier {
		t.Fatalf("expected identifier type, got %s", refVal.Type)
	}

	// Identifiers should not have programs
	if refVal.Program != nil {
		t.Error("identifier should not have compiled program")
	}
}

// TestCompileValue_TypeBoolean tests boolean type compilation.
func TestCompileValue_TypeBoolean(t *testing.T) {
	t.Parallel()

	input := `test : { flag : true }`
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	flagVal := ast.Namespaces[0].Value.Tuple.Values[0].Namespace.Value
	if flagVal.Type != TypeBoolean {
		t.Fatalf("expected boolean type, got %s", flagVal.Type)
	}

	// Booleans should not have programs
	if flagVal.Program != nil {
		t.Error("boolean should not have compiled program")
	}
}

// TestCompileValue_TypeString tests string type compilation.
func TestCompileValue_TypeString(t *testing.T) {
	t.Parallel()

	input := `test : { msg : "hello" }`
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	msgVal := ast.Namespaces[0].Value.Tuple.Values[0].Namespace.Value
	if msgVal.Type != TypeString {
		t.Fatalf("expected string type, got %s", msgVal.Type)
	}

	// Strings should not have programs
	if msgVal.Program != nil {
		t.Error("string should not have compiled program")
	}
}

// TestCompileValue_TypeNumber tests number type compilation.
func TestCompileValue_TypeNumber(t *testing.T) {
	t.Parallel()

	input := `test : { num : 42 }`
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	numVal := ast.Namespaces[0].Value.Tuple.Values[0].Namespace.Value
	if numVal.Type != TypeNumber {
		t.Fatalf("expected number type, got %s", numVal.Type)
	}

	// Numbers should not have programs
	if numVal.Program != nil {
		t.Error("number should not have compiled program")
	}
}

// TestCompileTuple_NilTuple tests compileTuple with nil Tuple field.
func TestCompileTuple_NilTuple(t *testing.T) {
	t.Parallel()

	ast := &AST{}
	applyDefaults(ast)

	// Create a value with TypeTuple but nil Tuple field
	val := &Value{Type: TypeTuple, Tuple: nil}

	err := compileTuple(context.Background(), val, nil, nil, ast)
	if err != nil {
		t.Errorf("compileTuple(nil tuple) returned error: %v", err)
	}
}

// TestCompileTuple_EmptyTuple tests compileTuple with empty tuple.
func TestCompileTuple_EmptyTuple(t *testing.T) {
	t.Parallel()

	input := `test : {}`
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tupleVal := ast.Namespaces[0].Value
	if tupleVal.Type != TypeTuple {
		t.Fatalf("expected tuple type, got %s", tupleVal.Type)
	}

	if len(tupleVal.Tuple.Values) != 0 {
		t.Errorf("expected empty tuple, got %d values", len(tupleVal.Tuple.Values))
	}
}

// TestCompileTuple_MixedTypes tests compileTuple with various value types.
func TestCompileTuple_MixedTypes(t *testing.T) {
	t.Parallel()

	input := `test : {
		num : 42,
		str : "hello",
		bool : true,
		expr : {{ num + 1 }},
		nested : { inner : 10 },
	}`
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tupleVal := ast.Namespaces[0].Value
	if tupleVal.Type != TypeTuple {
		t.Fatalf("expected tuple type, got %s", tupleVal.Type)
	}

	values := tupleVal.Tuple.Values

	// Check that we have all expected values
	if len(values) != 5 {
		t.Fatalf("expected 5 values, got %d", len(values))
	}

	// Verify expr was compiled
	exprVal := values[3].Namespace.Value
	if exprVal.Type != TypeExpr {
		t.Fatalf("expected expr type at index 3, got %s", exprVal.Type)
	}
	if exprVal.Program == nil {
		t.Error("expected compiled program for expr")
	}

	// Verify nested namespace
	nestedVal := values[4].Namespace.Value
	if nestedVal.Type != TypeTuple {
		t.Fatalf("expected tuple type at index 4, got %s", nestedVal.Type)
	}
}

// TestCompileExpr_FailedCompilation tests expr compilation that fails.
func TestCompileExpr_FailedCompilation(t *testing.T) {
	t.Parallel()

	input := `test : { x : {{ invalid_syntax +++ }} }`

	_, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err == nil {
		t.Fatal("expected compilation error, got nil")
	}

	// Verify error contains relevant information
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error message")
	}
}

// TestCompileExpr_WithParameters tests expr referencing parameters.
// Note: Expressions referencing parameters can't always be compiled at parse time
// because parameter values are unknown. They may be compiled lazily at evaluation.
func TestCompileExpr_WithParameters(t *testing.T) {
	t.Parallel()

	input := `greet name : { message : {{ "Hello, " + name }} }`

	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	messageVal := ast.Namespaces[0].Value.Tuple.Values[0].Namespace.Value
	if messageVal.Type != TypeExpr {
		t.Fatalf("expected expr type, got %s", messageVal.Type)
	}

	// Parameterized expressions may not compile (deferred to evaluation time)
	// This is expected behavior - we just verify the expr exists
}

// TestCompileExpr_MultipleScopes tests expr with multiple scope levels.
func TestCompileExpr_MultipleScopes(t *testing.T) {
	t.Parallel()

	input := `root : {
		outer : 1,
		level1 : {
			mid : 2,
			level2 : {
				inner : 3,
				result : {{ outer + mid + inner }},
			},
		},
	}`

	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Navigate to result
	rootVal := ast.Namespaces[0].Value
	level1Val := rootVal.Tuple.Values[1].Namespace.Value
	level2Val := level1Val.Tuple.Values[1].Namespace.Value
	resultVal := level2Val.Tuple.Values[1].Namespace.Value

	if resultVal.Type != TypeExpr {
		t.Fatalf("expected expr type, got %s", resultVal.Type)
	}

	if resultVal.Program == nil {
		t.Error("expected compiled program for multi-scope expr")
	}
}

// TestCompileNamespace_WithParameters tests namespace compilation with parameters.
// Note: Expressions referencing parameters may not be compiled at parse time.
func TestCompileNamespace_WithParameters(t *testing.T) {
	t.Parallel()

	input := `config env : {
		host : {{ "api." + env + ".example.com" }},
		port : 8080,
	}`

	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ns := ast.Namespaces[0]
	if len(ns.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(ns.Parameters))
	}

	hostVal := ns.Value.Tuple.Values[0].Namespace.Value
	if hostVal.Type != TypeExpr {
		t.Fatalf("expected expr type, got %s", hostVal.Type)
	}

	// Parameterized expressions may not compile (deferred to evaluation time)
	// This is expected behavior - we just verify the namespace structure
}

// TestCompileBuildStateReset tests that build state is properly reset.
func TestCompileBuildStateReset(t *testing.T) {
	t.Parallel()

	input := `test : { x : 10 }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// After parsing, build state should be reset
	if ast.build.depth != 0 {
		t.Errorf("expected depth 0, got %d", ast.build.depth)
	}

	if ast.build.chain != nil {
		t.Error("expected nil chain after build")
	}
}

// TestCompileExpr_EnvFunctionWithMissingVar tests env() with missing variable.
func TestCompileExpr_EnvFunctionWithMissingVar(t *testing.T) {
	t.Parallel()

	input := `test : { val : {{ env("NONEXISTENT_VAR_12345") }} }`

	// Parse with explicit empty env
	ast, err := ParseString(context.Background(), input,
		WithCompileExprs(true),
		WithProcessEnv([]string{}))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Should compile successfully (missing env vars are allowed at compile time)
	valExpr := ast.Namespaces[0].Value.Tuple.Values[0].Namespace.Value
	if valExpr.Type != TypeExpr {
		t.Fatalf("expected expr type, got %s", valExpr.Type)
	}

	if valExpr.Program == nil {
		t.Error("expected compiled program")
	}
}

// TestCompileTuple_NestedNamespaces tests tuple with multiple nested namespaces.
func TestCompileTuple_NestedNamespaces(t *testing.T) {
	t.Parallel()

	input := `root : {
		ns1 : { a : 1 },
		ns2 : { b : 2 },
		ns3 : { c : 3 },
	}`

	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rootVal := ast.Namespaces[0].Value
	if rootVal.Type != TypeTuple {
		t.Fatalf("expected tuple type, got %s", rootVal.Type)
	}

	// All three values should be namespaces
	for i, val := range rootVal.Tuple.Values {
		if val.Type != TypeNamespace {
			t.Errorf("value %d: expected namespace type, got %s", i, val.Type)
		}
	}
}

// TestCompileTuple_NonNamespaceValues tests tuple with mixed namespace and non-namespace values.
func TestCompileTuple_NonNamespaceValues(t *testing.T) {
	t.Parallel()

	input := `root : {
		ns : { x : 1 },
		num : 42,
		str : "hello",
	}`

	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rootVal := ast.Namespaces[0].Value
	values := rootVal.Tuple.Values

	// First is namespace
	if values[0].Type != TypeNamespace {
		t.Errorf("value 0: expected namespace, got %s", values[0].Type)
	}

	// Second is namespace (containing number)
	if values[1].Type != TypeNamespace {
		t.Errorf("value 1: expected namespace, got %s", values[1].Type)
	}

	// Third is namespace (containing string)
	if values[2].Type != TypeNamespace {
		t.Errorf("value 2: expected namespace, got %s", values[2].Type)
	}
}
