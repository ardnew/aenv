package lang

import (
	"strconv"

	"github.com/ardnew/aenv/lang/token"
)

// Package-level value constructors provide a programmatic API for constructing
// AST nodes without parsing source text. These replace the Builder type.

// NewString creates a string literal [Value].
func NewString(s string) *Value {
	return &Value{
		Type:  TypeString,
		Token: newToken("string_literal", strconv.Quote(s)),
	}
}

// NewNumber creates a number literal [Value].
func NewNumber(n string) *Value {
	return &Value{
		Type:  TypeNumber,
		Token: newToken("number_literal", n),
	}
}

// NewBool creates a boolean literal [Value].
func NewBool(v bool) *Value {
	return &Value{
		Type:  TypeBoolean,
		Token: newToken("boolean_literal", strconv.FormatBool(v)),
	}
}

// NewIdentifier creates an identifier [Value].
func NewIdentifier(name string) *Value {
	return &Value{
		Type:  TypeIdentifier,
		Token: newToken("identifier", name),
	}
}

// NewExpr creates an expression literal [Value].
func NewExpr(expr string) *Value {
	return &Value{
		Type:  TypeExpr,
		Token: newToken("expr_literal", expr),
	}
}

// NewTuple creates a tuple [Value].
func NewTuple(values ...*Value) *Value {
	return &Value{
		Type:  TypeTuple,
		Tuple: &Tuple{Values: values},
	}
}

// NewNamespace creates a namespace [Value].
func NewNamespace(
	identifier string,
	params []*Value,
	value *Value,
) *Value {
	return &Value{
		Type: TypeNamespace,
		Namespace: &Namespace{
			Identifier: newToken("identifier", identifier),
			Parameters: params,
			Value:      value,
		},
	}
}

// newToken creates a [token.Token] with given type and literal.
//
// For programmatic AST construction, extents default to covering
// the entire literal string (lext=0, rext=len(lit)).
func newToken(typ, lit string) *token.Token {
	tokType, ok := token.IDToType[typ]
	if !ok {
		tokType = token.T_0 // Error type
	}

	input := []rune(lit)

	return token.New(tokType, 0, len(input), input)
}
