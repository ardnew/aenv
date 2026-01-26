package lang

import (
	"bytes"
	"strings"
	"testing"
)

// TestParseString_SimpleDefinition tests parsing a simple definition.
func TestParseString_SimpleDefinition(t *testing.T) {
	input := `test : { foo : 123 }`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(ast.Definitions))
	}

	def := ast.Definitions[0]
	if def.Identifier.LiteralString() != "test" {
		t.Errorf("expected definition 'test', got %q", def.Identifier.LiteralString())
	}

	if def.Value.Type != TypeTuple {
		t.Fatalf("expected tuple value, got %s", def.Value.Type)
	}
	if len(def.Value.Tuple.Aggregate) != 1 {
		t.Fatalf("expected 1 member, got %d", len(def.Value.Tuple.Aggregate))
	}

	value := def.Value.Tuple.Aggregate[0]

	if value.Type != TypeDefinition {
		t.Fatalf("expected member to be Definition, got %v", value.Type)
	}

	inner := value.Definition

	if inner.Identifier.LiteralString() != "foo" {
		t.Errorf("expected member 'foo', got %q", inner.Identifier.LiteralString())
	}

	if inner.Value.Type != TypeNumber {
		t.Errorf("expected Number value type, got %v", inner.Value.Type)
	}
}

// TestParseString_MultipleDefinitions tests parsing multiple definitions.
func TestParseString_MultipleDefinitions(t *testing.T) {
	input := `
		ns1 : { a : 1 }
		ns2 : { b : 2 }
	`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(ast.Definitions))
	}

	if ast.Definitions[0].Identifier.LiteralString() != "ns1" {
		t.Errorf("expected first definition 'ns1', got %q", ast.Definitions[0].Identifier.LiteralString())
	}

	if ast.Definitions[1].Identifier.LiteralString() != "ns2" {
		t.Errorf("expected second definition 'ns2', got %q", ast.Definitions[1].Identifier.LiteralString())
	}
}

// TestParseString_DefinitionWithParameters tests parsing definition parameters.
func TestParseString_DefinitionWithParameters(t *testing.T) {
	input := `test "arg1" 123 : { }`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]

	if len(def.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(def.Parameters))
	}

	if def.Parameters[0].Type != TypeString {
		t.Errorf("expected first parameter to be String, got %v", def.Parameters[0].Type)
	}

	if def.Parameters[1].Type != TypeNumber {
		t.Errorf("expected second parameter to be Number, got %v", def.Parameters[1].Type)
	}
}

// TestParseString_AllValueTypes tests all value types.
func TestParseString_AllValueTypes(t *testing.T) {
	input := `test : {
		id : myvar,
		num : 42,
		str : "hello",
		code : {{ some code }},
		bool : true,
		tuple : { inner : 1 },
		def 123 : nested "param" : { value : 99 },
	}`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	if def.Value.Type != TypeTuple {
		t.Fatalf("expected tuple value, got %s", def.Value.Type)
	}
	aggregate := def.Value.Tuple.Aggregate

	if len(aggregate) != 7 {
		t.Fatalf("expected 7 aggregate, got %d", len(aggregate))
	}

	tests := []struct {
		name       string
		params     []Type
		innerType  Type
	}{
		{"id",  nil, TypeIdentifier},
		{"num", nil, TypeNumber},
		{"str", nil, TypeString},
		{"code", nil, TypeExpr},
		{"bool", nil, TypeBoolean},
		{"tuple", nil, TypeTuple},
		{"def", []Type{TypeNumber}, TypeDefinition},
	}

	for i, tt := range tests {
		// Every Value in test is a Definition
		if aggregate[i].Type != TypeDefinition {
			t.Errorf("expected member %q to be Definition, got %v", tt.name, aggregate[i].Type)
			continue
		}

		inner := aggregate[i].Definition

		if inner.Value.Type != tt.innerType {
			t.Errorf("expected member %q inner type %v, got %v", tt.name, tt.innerType, inner.Value.Type)
			continue
		}

		if len(inner.Parameters) != len(tt.params) {
			t.Errorf("expected member %q to have %d parameters, got %d", tt.name, len(tt.params), len(inner.Parameters))
			continue
		}

		for j, pType := range tt.params {
			if inner.Parameters[j].Type != pType {
				t.Errorf("expected member %q parameter %d to be %v, got %v", tt.name, j, pType, inner.Parameters[j].Type)
				break
			}
		}
	}
}

// TestAST_Print tests the AST print functionality.
func TestAST_Print(t *testing.T) {
	input := `test : { foo : 123 }`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var buf bytes.Buffer
	ast.Print(&buf)

	output := buf.String()
	if !strings.Contains(output, "Definition: test") {
		t.Errorf("output doesn't contain definition name: %s", output)
	}
	// Tuples contain Values which can be Definitions (shown as nested)
	if !strings.Contains(output, "Definition: foo") {
		t.Errorf("output doesn't contain nested definition: %s", output)
	}
	if !strings.Contains(output, "Number: 123") {
		t.Errorf("output doesn't contain value: %s", output)
	}
}

// TestParseString_RecursiveDefinition tests parsing recursive definitions.
func TestParseString_RecursiveDefinition(t *testing.T) {
	input := `test : { nested : inner "arg" : { value : 42 } }`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(ast.Definitions))
	}

	def := ast.Definitions[0]
	if def.Value.Type != TypeTuple {
		t.Fatalf("expected tuple value, got %s", def.Value.Type)
	}
	if len(def.Value.Tuple.Aggregate) != 1 {
		t.Fatalf("expected 1 member, got %d", len(def.Value.Tuple.Aggregate))
	}

	value := def.Value.Tuple.Aggregate[0]

	if value.Type != TypeDefinition {
		t.Fatalf("expected Definition value type, got %v", value.Type)
	}

	nested := value.Definition



	if nested.Identifier.LiteralString() != "nested" {
		t.Errorf("expected value 'nested', got %q", nested.Identifier.LiteralString())
	}

	if nested.Value.Type != TypeDefinition {
		t.Fatalf("expected nested value to be Definition, got %v", nested.Value.Type)
	}

	inner := nested.Value.Definition

	if inner.Identifier.LiteralString() != "inner" {
		t.Errorf("expected inner definition 'inner', got %q", inner.Identifier.LiteralString())
	}

	if len(inner.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(inner.Parameters))
	}

	if inner.Parameters[0].Type != TypeString {
		t.Errorf("expected parameter to be String, got %v", inner.Parameters[0].Type)
	}

	if inner.Value.Type != TypeTuple {
		t.Fatalf("expected inner value to be Tuple, got %v", inner.Value.Type)
	}
}

// TestParseString_EmptyInput tests parsing empty input.
func TestParseString_EmptyInput(t *testing.T) {
	input := ``

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ast.Definitions) != 0 {
		t.Errorf("expected 0 definitions for empty input, got %d", len(ast.Definitions))
	}
}

// TestParseString_InvalidInput tests that invalid input returns an error.
func TestParseString_InvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing tuple", `test ::`},
		{"unclosed tuple", `test : {`},
		{"unclosed string", `test : { x : "unclosed }`},
		{"invalid syntax", `test : { : }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(tt.input)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}
		})
	}
}
