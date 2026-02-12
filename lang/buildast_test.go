package lang

import (
	"context"
	"testing"
)

// TestBuildAST_SimpleNamespace tests basic buildAST functionality.
func TestBuildAST_SimpleNamespace(t *testing.T) {
	t.Parallel()

	input := `test : { x : 10 }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(ast.Namespaces))
	}

	if ast.Namespaces[0].Identifier.LiteralString() != "test" {
		t.Errorf("expected namespace 'test', got %q", ast.Namespaces[0].Identifier.LiteralString())
	}
}

// TestBuildAST_MultipleNamespaces tests buildAST with multiple top-level namespaces.
func TestBuildAST_MultipleNamespaces(t *testing.T) {
	t.Parallel()

	input := `
		first : { a : 1 };
		second : { b : 2 };
		third : { c : 3 }
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Namespaces) != 3 {
		t.Fatalf("expected 3 namespaces, got %d", len(ast.Namespaces))
	}
}

// TestBuildAST_EmptyInput tests buildAST with empty input.
func TestBuildAST_EmptyInput(t *testing.T) {
	t.Parallel()

	input := ``

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Namespaces) != 0 {
		t.Fatalf("expected 0 namespaces, got %d", len(ast.Namespaces))
	}
}

// TestBuildAST_WithOptions tests that buildAST correctly applies options.
func TestBuildAST_WithOptions(t *testing.T) {
	t.Parallel()

	input := `test : { x : {{ 1 + 2 }} }`

	// Parse with compile option
	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Verify the option was applied (expression should be compiled)
	exprVal := ast.Namespaces[0].Value.Tuple.Values[0].Namespace.Value
	if exprVal.Type != TypeExpr {
		t.Fatalf("expected expr type, got %s", exprVal.Type)
	}

	if exprVal.Program == nil {
		t.Error("expected compiled program with WithCompileExprs option")
	}
}

// TestBuildAST_BuildStateReset tests that build state is properly reset after parsing.
func TestBuildAST_BuildStateReset(t *testing.T) {
	t.Parallel()

	input := `test : { nested : { deep : 42 } }`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// After buildAST completes, build state should be reset
	if ast.build.depth != 0 {
		t.Errorf("expected depth 0 after build, got %d", ast.build.depth)
	}

	if ast.build.chain != nil {
		t.Error("expected nil chain after build")
	}
}

// TestBuildAST_NestedStructure tests buildAST with deeply nested structures.
func TestBuildAST_NestedStructure(t *testing.T) {
	t.Parallel()

	input := `root : {
		level1 : {
			level2 : {
				level3 : {
					value : "deep"
				}
			}
		}
	}`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Verify structure was built correctly
	root := ast.Namespaces[0]
	if root.Identifier.LiteralString() != "root" {
		t.Errorf("expected root name 'root', got %q", root.Identifier.LiteralString())
	}

	// Navigate through nested structure
	level1 := root.Value.Tuple.Values[0].Namespace
	if level1.Identifier.LiteralString() != "level1" {
		t.Errorf("expected level1 name 'level1', got %q", level1.Identifier.LiteralString())
	}

	level2 := level1.Value.Tuple.Values[0].Namespace
	if level2.Identifier.LiteralString() != "level2" {
		t.Errorf("expected level2 name 'level2', got %q", level2.Identifier.LiteralString())
	}

	level3 := level2.Value.Tuple.Values[0].Namespace
	if level3.Identifier.LiteralString() != "level3" {
		t.Errorf("expected level3 name 'level3', got %q", level3.Identifier.LiteralString())
	}
}

// TestBuildAST_WithParameters tests buildAST with parameterized namespaces.
func TestBuildAST_WithParameters(t *testing.T) {
	t.Parallel()

	input := `greet name age : {
		message : "Hello",
		info : "Person info"
	}`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ns := ast.Namespaces[0]
	if len(ns.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(ns.Parameters))
	}

	if ns.Parameters[0].Token.LiteralString() != "name" {
		t.Errorf("expected parameter 'name', got %q", ns.Parameters[0].Token.LiteralString())
	}

	if ns.Parameters[1].Token.LiteralString() != "age" {
		t.Errorf("expected parameter 'age', got %q", ns.Parameters[1].Token.LiteralString())
	}
}

// TestBuildAST_AllValueTypes tests buildAST with all value types.
func TestBuildAST_AllValueTypes(t *testing.T) {
	t.Parallel()

	input := `test : {
		str : "hello",
		num : 42,
		bool : true,
		ident : someid,
		expr : {{ 1 + 2 }},
		tuple : { a : 1 },
	}`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	values := ast.Namespaces[0].Value.Tuple.Values

	if len(values) != 6 {
		t.Fatalf("expected 6 values, got %d", len(values))
	}

	// Verify each type
	expectedTypes := []Type{
		TypeString,
		TypeNumber,
		TypeBoolean,
		TypeIdentifier,
		TypeExpr,
		TypeTuple,
	}

	for i, expected := range expectedTypes {
		actual := values[i].Namespace.Value.Type
		if actual != expected {
			t.Errorf("value %d: expected type %s, got %s", i, expected, actual)
		}
	}
}

// TestBuildAST_WithComments tests that buildAST handles comments correctly.
func TestBuildAST_WithComments(t *testing.T) {
	t.Parallel()

	input := `
		// Line comment
		test : {
			/* Block comment */
			x : 10,
			# Hash comment
			y : 20
		}
	`

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(ast.Namespaces))
	}

	// Comments should be ignored
	values := ast.Namespaces[0].Value.Tuple.Values
	if len(values) != 2 {
		t.Errorf("expected 2 values (comments ignored), got %d", len(values))
	}
}

// TestBuildAST_ErrorInBuildNamespaces tests error propagation from buildNamespaces.
func TestBuildAST_ErrorInBuildNamespaces(t *testing.T) {
	t.Parallel()

	// Invalid syntax that will cause buildNamespaces to fail
	input := `test : { unclosed`

	_, err := ParseString(context.Background(), input)
	if err == nil {
		t.Fatal("expected error from invalid syntax, got nil")
	}
}

// TestBuildAST_WithLogger tests that buildAST works with logger.
func TestBuildAST_WithLogger(t *testing.T) {
	t.Parallel()

	input := `test : { x : 10 }`

	// Parse with logger (default logger is applied)
	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Logger is initialized by default - just verify parsing worked
	if len(ast.Namespaces) == 0 {
		t.Error("expected at least one namespace")
	}
}

// TestBuildAST_ComplexMixedStructure tests buildAST with complex real-world-like structure.
func TestBuildAST_ComplexMixedStructure(t *testing.T) {
	t.Parallel()

	input := `
		config env : {
			database : {
				host : "localhost",
				port : 5432,
				name : {{ env + "_db" }},
			},
			api : {
				endpoint : "https://api.example.com",
				timeout : 30,
			},
			features : {
				enabled : true,
				flags : {
					beta : false,
					experimental : true,
				},
			},
		};

		settings : {
			debug : false,
			version : "1.0.0",
		}
	`

	ast, err := ParseString(context.Background(), input, WithCompileExprs(true))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Namespaces) != 2 {
		t.Fatalf("expected 2 top-level namespaces, got %d", len(ast.Namespaces))
	}

	// Verify config namespace
	config := ast.Namespaces[0]
	if config.Identifier.LiteralString() != "config" {
		t.Errorf("expected namespace 'config', got %q", config.Identifier.LiteralString())
	}

	if len(config.Parameters) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(config.Parameters))
	}

	// Verify settings namespace
	settings := ast.Namespaces[1]
	if settings.Identifier.LiteralString() != "settings" {
		t.Errorf("expected namespace 'settings', got %q", settings.Identifier.LiteralString())
	}
}
