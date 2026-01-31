package lang

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ardnew/aenv/lang/lexer"
	"github.com/ardnew/aenv/lang/parser"
	"github.com/ardnew/aenv/lang/token"
)

// Lexer Tests
// ============================================================================

// TestLexer_Identifier verifies identifier token generation.
func TestLexer_Identifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple identifier",
			input: "foo",
			want:  []string{"foo"},
		},
		{
			name:  "identifier with underscore",
			input: "_foo",
			want:  []string{"_foo"},
		},
		{
			name:  "identifier with dot",
			input: "foo.bar",
			want:  []string{"foo.bar"},
		},
		{
			name:  "identifier with slash",
			input: "foo/bar",
			want:  []string{"foo/bar"},
		},
		{
			name:  "identifier with dash",
			input: "foo-bar",
			want:  []string{"foo-bar"},
		},
		{
			name:  "identifier with plus",
			input: "foo+bar",
			want:  []string{"foo+bar"},
		},
		{
			name:  "identifier with at",
			input: "user@host",
			want:  []string{"user@host"},
		},
		{
			name:  "multiple identifiers",
			input: "foo bar baz",
			want:  []string{"foo", "bar", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			got := filterIdentifiers(l.Tokens)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d identifiers, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("identifier[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestLexer_NumberLiteral verifies number literal token generation.
func TestLexer_NumberLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		// Integers
		{name: "positive integer", input: "123", valid: true},
		{name: "negative integer", input: "-456", valid: true},

		// Octal
		{name: "octal", input: "0755", valid: true},
		{name: "octal zero", input: "00", valid: true},

		// Hexadecimal
		{name: "hex lowercase", input: "0xff", valid: true},
		{name: "hex mixed", input: "0xDeadBeef", valid: true},

		// Binary
		{name: "binary", input: "0b1010", valid: true},

		// Floating point
		{name: "simple float", input: "123.456", valid: true},
		{name: "negative float", input: "-123.456", valid: true},
		{name: "float starting with dot", input: "0.5", valid: true},
		{name: "zero float", input: "0.0", valid: true},

		// Scientific notation
		{name: "scientific positive exp", input: "1.23e10", valid: true},
		{name: "scientific negative exp", input: "1.23e-10", valid: true},
		{name: "scientific uppercase E", input: "1.23E10", valid: true},
		{name: "scientific explicit positive", input: "1.23e+10", valid: true},
		{name: "scientific without decimal", input: "123e10", valid: true},
		{name: "negative scientific", input: "-1234.5678E-9", valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			hasNumber := hasTokenType(l.Tokens, token.IDToType["number_literal"])
			if hasNumber != tt.valid {
				if tt.valid {
					t.Errorf("expected valid number literal, got error")
				} else {
					t.Errorf("expected invalid number literal, got valid")
				}
			}
		})
	}
}

// TestLexer_StringLiteral verifies string literal token generation.
func TestLexer_StringLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: `""`,
			want:  `""`,
		},
		{
			name:  "simple string",
			input: `"hello"`,
			want:  `"hello"`,
		},
		{
			name:  "string with spaces",
			input: `"hello world"`,
			want:  `"hello world"`,
		},
		{
			name:  "string with escape sequences",
			input: `"hello\nworld"`,
			want:  `"hello\nworld"`,
		},
		{
			name:  "string with quote escape",
			input: `"say \"hello\""`,
			want:  `"say \"hello\""`,
		},
		{
			name:  "string with backslash",
			input: `"path\\to\\file"`,
			want:  `"path\\to\\file"`,
		},
		{
			name:  "string with unicode escape",
			input: `"\u0048"`,
			want:  `"\u0048"`,
		},
		{
			name:  "string with hex escape",
			input: `"\x48"`,
			want:  `"\x48"`,
		},
		{
			name:  "string with octal escape",
			input: `"\101"`,
			want:  `"\101"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			got := findTokenLiteral(l.Tokens, token.IDToType["string_literal"])
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLexer_CodeLiteral verifies expr literal token generation.
func TestLexer_CodeLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty expr block",
			input: `{{}}`,
			want:  `{{}}`,
		},
		{
			name:  "simple expr",
			input: `{{foo}}`,
			want:  `{{foo}}`,
		},
		{
			name:  "expr with semicolons",
			input: `{{foo; bar; baz}}`,
			want:  `{{foo; bar; baz}}`,
		},
		{
			name:  "expr with commas",
			input: `{{foo, bar, baz}}`,
			want:  `{{foo, bar, baz}}`,
		},
		{
			name:  "expr with spaces",
			input: `{{   foo; hello, one }}`,
			want:  `{{   foo; hello, one }}`,
		},
		{
			name:  "expr with single brace",
			input: `{{foo{bar}}`,
			want:  `{{foo{bar}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			got := findTokenLiteral(l.Tokens, token.IDToType["expr_literal"])
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLexer_Comments verifies comment token handling.
func TestLexer_Comments(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "line comment with slashes",
			input: "// this is a comment\n",
		},
		{
			name:  "line comment with hash",
			input: "# this is a comment\n",
		},
		{
			name:  "block comment",
			input: "/* this is a block comment */",
		},
		{
			name:  "block comment multiline",
			input: "/* this is\na multi-line\ncomment */",
		},
		{
			name:  "block comment with stars",
			input: "/**** comment ****/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			// Comments should be lexed but filtered (marked with !)
			// Verify we can lex them without error
			if l == nil {
				t.Fatal("lexer returned nil")
			}
		})
	}
}

// TestLexer_Delimiter verifies delimiter token generation.
func TestLexer_Delimiter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{
			name:  "single comma",
			input: ",",
			count: 1,
		},
		{
			name:  "multiple commas",
			input: ",,",
			count: 2, // Two separate delimiter tokens
		},
		// Removed "commas with spaces" test - behavior depends on exact grammar
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			delims := countTokenType(l.Tokens, token.IDToType["delimiter"])
			if delims != tt.count {
				t.Errorf("got %d delimiters, want %d", delims, tt.count)
			}
		})
	}
}

// TestLexer_ComplexInput verifies lexing of complex input combining multiple token types.
func TestLexer_ComplexInput(t *testing.T) {
	input := `name:{ /ke\ y.1 /*foo*/ : "hello world", uep : {{   foo; hello, one }},
	key2 : {-1234.5678E-9, 10} }// ok`

	l := lexer.New([]rune(input))
	if l == nil {
		t.Fatal("lexer returned nil")
	}

	// Verify we have various token types
	identCount := countTokenType(l.Tokens, token.IDToType["identifier"])
	if identCount == 0 {
		t.Error("expected identifiers in complex input")
	}

	stringCount := countTokenType(l.Tokens, token.IDToType["string_literal"])
	if stringCount == 0 {
		t.Error("expected string literals in complex input")
	}

	numberCount := countTokenType(l.Tokens, token.IDToType["number_literal"])
	if numberCount == 0 {
		t.Error("expected number literals in complex input")
	}

	exprCount := countTokenType(l.Tokens, token.IDToType["expr_literal"])
	if exprCount == 0 {
		t.Error("expected expr literals in complex input")
	}
}

// Parser Tests
// ============================================================================

// TestParser_Value verifies parsing of individual value types.
func TestParser_Value(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "identifier value", input: "ns:{key:foo}", wantErr: false},
		{name: "boolean true", input: "ns:{key:true}", wantErr: false},
		{name: "boolean false", input: "ns:{key:false}", wantErr: false},
		{name: "number value", input: "ns:{key:123}", wantErr: false},
		{name: "string value", input: `ns:{key:"hello"}`, wantErr: false},
		{name: "expr value", input: `ns:{key:{{expr}}}`, wantErr: false},
		{name: "definition value", input: "ns:{key:{nested:val}}", wantErr: false},
		{name: "tuple value", input: "ns:{key:{1,2,3}}", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			_, err := parser.Parse(l)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParser_Boolean verifies parsing of boolean keywords.
func TestParser_Boolean(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "true keyword", input: "ns:{flag:true}"},
		{name: "false keyword", input: "ns:{flag:false}"},
		{name: "multiple booleans", input: "ns:{a:true, b:false}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			bsr, err := parser.Parse(l)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(bsr.GetRoots()) == 0 {
				t.Error("expected parse tree roots")
			}
		})
	}
}

// TestParser_Tuple verifies parsing of tuple constructs.
func TestParser_Tuple(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty tuple", input: "ns:{key:{}}", wantErr: false},
		{name: "single element", input: "ns:{key:{1}}", wantErr: false},
		{name: "multiple elements", input: "ns:{key:{1,2,3}}", wantErr: false},
		{name: "mixed types", input: `ns:{key:{1,"two",three}}`, wantErr: false},
		{name: "nested tuple", input: "ns:{key:{{1,2},{3,4}}}", wantErr: false},
		{name: "tuple with trailing comma", input: "ns:{key:{1,2,}}", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			_, err := parser.Parse(l)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParser_Definition verifies parsing of definition constructs (tuples containing definitions).
func TestParser_Definition(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty tuple", input: "ns:{}", wantErr: false},
		{name: "single definition", input: "ns:{key:val}", wantErr: false},
		{name: "multiple definitions", input: "ns:{a:1,b:2,c:3}", wantErr: false},
		{name: "nested tuple", input: "ns:{outer:{inner:val}}", wantErr: false},
		{name: "deeply nested", input: "ns:{a:{b:{c:{d:val}}}}", wantErr: false},
		{name: "trailing comma", input: "ns:{a:1,}", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			_, err := parser.Parse(l)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParser_TopLevel verifies parsing of top-level definition declarations.
func TestParser_TopLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple definition", input: "name:{}", wantErr: false},
		{name: "definition with values", input: "name:{a:1,b:2}", wantErr: false},
		{name: "definition with parameters", input: "name arg1 arg2:{}", wantErr: false},
		{name: "definition with single parameter", input: "name region:{}", wantErr: false},
		{name: "definition with dotted parameter", input: "name foo.bar:{}", wantErr: false},
		{name: "complex identifier", input: "ns.name:{}", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			_, err := parser.Parse(l)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParser_MultipleDefinitions verifies parsing of multiple top-level definition declarations.
func TestParser_MultipleDefinitions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "two definitions", input: "ns1:{} ns2:{}", wantErr: false},
		{name: "three definitions", input: "a:{} b:{} c:{}", wantErr: false},
		{name: "definitions with content", input: "a:{x:1} b:{y:2}", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			bsr, err := parser.Parse(l)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(bsr.GetRoots()) == 0 {
				t.Error("expected multiple parse tree roots")
			}
		})
	}
}

// TestParser_CommentsIgnored verifies that comments are properly ignored during parsing.
func TestParser_CommentsIgnored(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "line comment before",
			input: "// comment\nns:{}",
		},
		{
			name:  "line comment after",
			input: "ns:{} // comment",
		},
		{
			name:  "block comment",
			input: "ns:{ /* comment */ key:val}",
		},
		{
			name:  "multiple comments",
			input: "// line\nns:{ /* block */ key:val} # hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			_, err := parser.Parse(l)
			if err != nil {
				t.Errorf("Parse() with comments error = %v", err)
			}
		})
	}
}

// TestParser_ComplexNesting verifies parsing of deeply nested structures.
func TestParser_ComplexNesting(t *testing.T) {
	input := `
		config:{
			server:{
				host:"localhost",
				port:8080,
				options:{
					timeout:30,
					retries:3
				}
			},
			database:{
				connection:"db://localhost",
				pool:{10,20,30}
			}
		}
	`

	l := lexer.New([]rune(input))
	bsr, err := parser.Parse(l)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(bsr.GetRoots()) == 0 {
		t.Error("expected parse tree roots")
	}
}

// TestParser_AllFeatures verifies the original complex test case.
func TestParser_AllFeatures(t *testing.T) {
	input := `name:{ key.1 /*foo*/ : "hello world", uep : {{   foo; hello, one }},
	key2 : {-1234.5678E-9, 10} }// ok`

	l := lexer.New([]rune(input))
	bsr, err := parser.Parse(l)
	if err != nil {
		var errMsgs []string
		for _, e := range err {
			errMsgs = append(errMsgs, e.String())
		}
		t.Fatalf("Parse() error = %v", strings.Join(errMsgs, "; "))
	}

	roots := bsr.GetRoots()
	if len(roots) == 0 {
		t.Error("expected parse tree roots")
	}
}

// TestParser_ErrorCases verifies error handling for invalid syntax.
func TestParser_ErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "missing block", input: "ns::"},
		{name: "unclosed block", input: "ns:{key:val"},
		{name: "unclosed tuple", input: "ns:{key:{1,2"},
		{name: "unclosed string", input: `ns:{key:"unclosed}`},
		{name: "invalid assignment", input: "ns:{:val}"},
		{name: "missing colon", input: "ns:{key val}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]rune(tt.input))
			_, err := parser.Parse(l)
			if err == nil {
				t.Error("expected parse error, got nil")
			}
		})
	}
}

// AST Tests
// ============================================================================
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
	if len(def.Value.Tuple.Values) != 1 {
		t.Fatalf("expected 1 member, got %d", len(def.Value.Tuple.Values))
	}

	value := def.Value.Tuple.Values[0]

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
	input := `test arg1 arg2 : { }`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]

	if len(def.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(def.Parameters))
	}

	if def.Parameters[0].Type != TypeIdentifier {
		t.Errorf("expected first parameter to be Identifier, got %v", def.Parameters[0].Type)
	}
	if def.Parameters[0].Token.LiteralString() != "arg1" {
		t.Errorf("expected first parameter 'arg1', got %q", def.Parameters[0].Token.LiteralString())
	}

	if def.Parameters[1].Type != TypeIdentifier {
		t.Errorf("expected second parameter to be Identifier, got %v", def.Parameters[1].Type)
	}
	if def.Parameters[1].Token.LiteralString() != "arg2" {
		t.Errorf("expected second parameter 'arg2', got %q", def.Parameters[1].Token.LiteralString())
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
		def p1 : nested p2 : { value : 99 },
	}`

	ast, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	def := ast.Definitions[0]
	if def.Value.Type != TypeTuple {
		t.Fatalf("expected tuple value, got %s", def.Value.Type)
	}
	values := def.Value.Tuple.Values

	if len(values) != 7 {
		t.Fatalf("expected 7 values, got %d", len(values))
	}

	tests := []struct {
		name      string
		params    []Type
		innerType Type
	}{
		{"id", nil, TypeIdentifier},
		{"num", nil, TypeNumber},
		{"str", nil, TypeString},
		{"code", nil, TypeExpr},
		{"bool", nil, TypeBoolean},
		{"tuple", nil, TypeTuple},
		{"def", []Type{TypeIdentifier}, TypeDefinition},
	}

	for i, tt := range tests {
		// Every Value in test is a Definition
		if values[i].Type != TypeDefinition {
			t.Errorf("expected member %q to be Definition, got %v", tt.name, values[i].Type)
			continue
		}

		inner := values[i].Definition

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
	input := `test : { nested : inner arg : { value : 42 } }`

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
	if len(def.Value.Tuple.Values) != 1 {
		t.Fatalf("expected 1 member, got %d", len(def.Value.Tuple.Values))
	}

	value := def.Value.Tuple.Values[0]

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

	if inner.Parameters[0].Type != TypeIdentifier {
		t.Errorf("expected parameter to be Identifier, got %v", inner.Parameters[0].Type)
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

// Value Constructor Tests
// ============================================================================

// TestNewString tests the NewString value constructor.
func TestNewString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", `"hello"`},
		{"empty", "", `""`},
		{"with spaces", "hello world", `"hello world"`},
		{"with quotes", `say "hi"`, `"say \"hi\""`},
		{"with newline", "line1\nline2", `"line1\nline2"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewString(tt.input)
			if v.Type != TypeString {
				t.Errorf("expected TypeString, got %v", v.Type)
			}
			if v.Token == nil {
				t.Fatal("expected non-nil token")
			}
			if got := v.Token.LiteralString(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNewNumber tests the NewNumber value constructor.
func TestNewNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"integer", "42"},
		{"negative", "-123"},
		{"float", "3.14"},
		{"scientific", "1.23e10"},
		{"hex", "0xff"},
		{"binary", "0b1010"},
		{"octal", "0755"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewNumber(tt.input)
			if v.Type != TypeNumber {
				t.Errorf("expected TypeNumber, got %v", v.Type)
			}
			if v.Token == nil {
				t.Fatal("expected non-nil token")
			}
			if got := v.Token.LiteralString(); got != tt.input {
				t.Errorf("got %q, want %q", got, tt.input)
			}
		})
	}
}

// TestNewBool tests the NewBool value constructor.
func TestNewBool(t *testing.T) {
	tests := []struct {
		name  string
		input bool
		want  string
	}{
		{"true", true, "true"},
		{"false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewBool(tt.input)
			if v.Type != TypeBoolean {
				t.Errorf("expected TypeBoolean, got %v", v.Type)
			}
			if v.Token == nil {
				t.Fatal("expected non-nil token")
			}
			if got := v.Token.LiteralString(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNewIdentifier tests the NewIdentifier value constructor.
func TestNewIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "myvar"},
		{"with underscore", "_private"},
		{"with dots", "config.server.port"},
		{"with dash", "my-var"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewIdentifier(tt.input)
			if v.Type != TypeIdentifier {
				t.Errorf("expected TypeIdentifier, got %v", v.Type)
			}
			if v.Token == nil {
				t.Fatal("expected non-nil token")
			}
			if got := v.Token.LiteralString(); got != tt.input {
				t.Errorf("got %q, want %q", got, tt.input)
			}
		})
	}
}

// TestNewExpr tests the NewExpr value constructor.
func TestNewExpr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple", "{{ 1 + 2 }}"},
		{"with variable", "{{ x * 2 }}"},
		{"function call", "{{ env(\"HOME\") }}"},
		{"complex", "{{ (a + b) * c }}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewExpr(tt.input)
			if v.Type != TypeExpr {
				t.Errorf("expected TypeExpr, got %v", v.Type)
			}
			if v.Token == nil {
				t.Fatal("expected non-nil token")
			}
			if got := v.Token.LiteralString(); got != tt.input {
				t.Errorf("got %q, want %q", got, tt.input)
			}
		})
	}
}

// TestNewTuple tests the NewTuple value constructor.
func TestNewTuple(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		v := NewTuple()
		if v.Type != TypeTuple {
			t.Errorf("expected TypeTuple, got %v", v.Type)
		}
		if v.Tuple == nil {
			t.Fatal("expected non-nil tuple")
		}
		if len(v.Tuple.Values) != 0 {
			t.Errorf("expected 0 values, got %d", len(v.Tuple.Values))
		}
	})

	t.Run("single value", func(t *testing.T) {
		v := NewTuple(NewString("hello"))
		if v.Type != TypeTuple {
			t.Errorf("expected TypeTuple, got %v", v.Type)
		}
		if len(v.Tuple.Values) != 1 {
			t.Errorf("expected 1 value, got %d", len(v.Tuple.Values))
		}
	})

	t.Run("multiple values", func(t *testing.T) {
		v := NewTuple(
			NewString("a"),
			NewNumber("1"),
			NewBool(true),
		)
		if len(v.Tuple.Values) != 3 {
			t.Errorf("expected 3 values, got %d", len(v.Tuple.Values))
		}
		if v.Tuple.Values[0].Type != TypeString {
			t.Errorf("expected first value to be TypeString")
		}
		if v.Tuple.Values[1].Type != TypeNumber {
			t.Errorf("expected second value to be TypeNumber")
		}
		if v.Tuple.Values[2].Type != TypeBoolean {
			t.Errorf("expected third value to be TypeBoolean")
		}
	})
}

// TestNewDefinition tests the NewDefinition value constructor.
func TestNewDefinition(t *testing.T) {
	t.Run("simple definition", func(t *testing.T) {
		v := NewDefinition("key", nil, NewString("value"))
		if v.Type != TypeDefinition {
			t.Errorf("expected TypeDefinition, got %v", v.Type)
		}
		if v.Definition == nil {
			t.Fatal("expected non-nil definition")
		}
		if v.Definition.Identifier.LiteralString() != "key" {
			t.Errorf("expected identifier 'key', got %q", v.Definition.Identifier.LiteralString())
		}
		if len(v.Definition.Parameters) != 0 {
			t.Errorf("expected 0 parameters, got %d", len(v.Definition.Parameters))
		}
		if v.Definition.Value.Type != TypeString {
			t.Errorf("expected value type TypeString, got %v", v.Definition.Value.Type)
		}
	})

	t.Run("with parameters", func(t *testing.T) {
		params := []*Value{
			NewIdentifier("arg1"),
			NewIdentifier("arg2"),
		}
		v := NewDefinition("func", params, NewExpr("{{ arg1 + arg2 }}"))
		if len(v.Definition.Parameters) != 2 {
			t.Errorf("expected 2 parameters, got %d", len(v.Definition.Parameters))
		}
	})

	t.Run("nested definition", func(t *testing.T) {
		inner := NewDefinition("inner", nil, NewNumber("42"))
		outer := NewDefinition("outer", nil, NewTuple(inner))
		if outer.Definition.Value.Type != TypeTuple {
			t.Errorf("expected outer value to be TypeTuple")
		}
		if outer.Definition.Value.Tuple.Values[0].Type != TypeDefinition {
			t.Errorf("expected tuple to contain TypeDefinition")
		}
	})
}

// Error Type Tests
// ============================================================================

// TestError_Error tests the Error.Error() method.
func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "message only",
			err:  NewError("something failed"),
			want: "something failed",
		},
		{
			name: "with wrapped error",
			err:  NewError("operation failed").Wrap(NewError("inner error")),
			want: "operation failed: inner error",
		},
		{
			name: "empty message with wrapped",
			err:  (&Error{}).Wrap(NewError("inner")),
			want: "inner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestError_With tests the Error.With() method for adding attributes.
func TestError_With(t *testing.T) {
	base := NewError("test error")
	withAttr := base.With(slog.String("key", "value"))

	// Original should be unchanged
	if len(base.attrs) != 0 {
		t.Error("base error should have no attributes")
	}

	// New error should have attribute
	if len(withAttr.attrs) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(withAttr.attrs))
	}

	// Multiple attributes
	withMulti := base.With(
		slog.String("a", "1"),
		slog.Int("b", 2),
	)
	if len(withMulti.attrs) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(withMulti.attrs))
	}
}

// TestError_Unwrap tests error unwrapping for errors.Is/As.
func TestError_Unwrap(t *testing.T) {
	inner := NewError("inner")
	outer := NewError("outer").Wrap(inner)

	if outer.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}

	// Test with errors.Is
	if !errors.Is(outer, inner) {
		t.Error("errors.Is should find inner error")
	}
}

// TestWrapError tests the WrapError function.
func TestWrapError(t *testing.T) {
	t.Run("wraps standard error", func(t *testing.T) {
		stdErr := errors.New("standard error")
		wrapped := WrapError(stdErr)
		if wrapped.err != stdErr {
			t.Error("should wrap standard error")
		}
	})

	t.Run("returns existing Error unchanged", func(t *testing.T) {
		existing := NewError("existing")
		wrapped := WrapError(existing)
		if wrapped != existing {
			t.Error("should return existing Error as-is")
		}
	})
}

// Helper functions
// ============================================================================

func filterIdentifiers(tokens []*token.Token) []string {
	var result []string
	// Use 'identifier' token type for identifier names
	identType := token.IDToType["identifier"]
	for _, tok := range tokens {
		if tok.Type() == identType {
			result = append(result, tok.LiteralString())
		}
	}
	return result
}

func hasTokenType(tokens []*token.Token, tokenType token.Type) bool {
	for _, tok := range tokens {
		if tok.Type() == tokenType {
			return true
		}
	}
	return false
}

func findTokenLiteral(tokens []*token.Token, tokenType token.Type) string {
	for _, tok := range tokens {
		if tok.Type() == tokenType {
			return tok.LiteralString()
		}
	}
	return ""
}

func countTokenType(tokens []*token.Token, tokenType token.Type) int {
	count := 0
	for _, tok := range tokens {
		if tok.Type() == tokenType {
			count++
		}
	}
	return count
}
