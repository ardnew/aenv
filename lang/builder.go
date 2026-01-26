package lang

import (
	"strconv"

	"github.com/ardnew/aenv/lang/token"
)

// Builder provides a programmatic API for constructing AST nodes without
// parsing source text. This is useful for generating formatted aenv files
// programmatically or for testing.
//
// Example:
//
//	b := lang.NewBuilder()
//	ast := b.AST(
//	    b.Define("config", nil,
//	        b.Tuple(
//	            b.Definition("key", nil, b.String("value")),
//	        ),
//	    ),
//	)
type Builder struct{}

// NewBuilder creates a new AST builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Define creates a definition [Value] and returns its [Value.Definition].
func (b *Builder) Define(
	identifier string,
	params []*Value,
	value *Value,
) *Definition {
	return b.Definition(identifier, params, value).Definition
}

// Definition creates a definition [Value].
func (b *Builder) Definition(
	identifier string,
	params []*Value,
	value *Value,
) *Value {
	return &Value{
		Type: TypeDefinition,
		Definition: &Definition{
			Identifier: b.Identifier(identifier).Token,
			Parameters: params,
			Value:      value,
		},
	}
}

// Tuple creates a tuple [Value].
func (b *Builder) Tuple(aggregate ...*Value) *Value {
	return &Value{
		Type:  TypeTuple,
		Tuple: &Tuple{Aggregate: aggregate},
	}
}

// Identifier creates an identifier [Value].
func (b *Builder) Identifier(name string) *Value {
	return &Value{
		Type:  TypeIdentifier,
		Token: b.makeToken("identifier", name),
	}
}

// String creates a string literal [Value].
func (b *Builder) String(s string) *Value {
	return &Value{
		Type:  TypeString,
		Token: b.makeToken("string_literal", strconv.Quote(s)),
	}
}

// Number creates a number literal [Value].
func (b *Builder) Number(n string) *Value {
	return &Value{
		Type:  TypeNumber,
		Token: b.makeToken("number_literal", n),
	}
}

// Bool creates a boolean literal [Value].
func (b *Builder) Bool(v bool) *Value {
	return &Value{
		Type:  TypeBoolean,
		Token: b.makeToken("boolean_literal", strconv.FormatBool(v)),
	}
}

// makeToken creates a [token.Token] with given type and literal.
//
// For programmatic AST construction, extents default to covering
// the entire literal string (lext=0, rext=len(lit)).
func (b *Builder) makeToken(
	typ string,
	lit string,
) *token.Token {
	tokType, ok := token.IDToType[typ]
	if !ok {
		tokType = token.T_0 // Error type
	}

	input := []rune(lit)

	return token.New(tokType, 0, len(input), input)
}

// AST creates an [AST] with the given [Definition]s.
func (b *Builder) AST(defs ...*Definition) *AST {
	return &AST{Definitions: defs}
}
