package lang

// NewExpr creates a new expression value from the given source text.
// The source is passed verbatim to expr-lang for compilation and evaluation.
//
// Examples:
//
//	NewExpr("42")                    // integer
//	NewExpr("\"hello\"")             // string (quoted)
//	NewExpr("true")                  // boolean
//	NewExpr("x + 1")                 // expression
//	NewExpr(`["a", "b", "c"]`)       // array
//	NewExpr(`{"key": "value"}`)      // map
func NewExpr(source string) *Value {
	return &Value{
		Kind:   KindExpr,
		Source: source,
	}
}

// NewBlock creates a new block value from the given namespace entries.
// Blocks evaluate to map[string]any.
func NewBlock(entries ...*Namespace) *Value {
	return &Value{
		Kind:    KindBlock,
		Entries: entries,
	}
}

// NewNamespace creates a new namespace with the given name, parameters, and
// value.
func NewNamespace(name string, params []Param, value *Value) *Namespace {
	return &Namespace{
		Name:   name,
		Params: params,
		Value:  value,
	}
}
