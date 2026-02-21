package lang

// FuncRef is returned by [AST.EvaluateExpr] when the evaluated expression
// resolves to a callable (a parameterized namespace or a builtin function)
// rather than a concrete value. It carries the function name and a
// human-readable signature for display purposes.
type FuncRef struct {
	// Name is the bare identifier of the function (e.g. "add").
	Name string
	// Signature is a human-readable call signature (e.g. "add(x, y)").
	// It is empty only when no parameter information could be derived.
	Signature string
}

// NewFuncRef creates a [FuncRef] with the given name and signature.
func NewFuncRef(name, sig string) *FuncRef {
	return &FuncRef{
		Name:      name,
		Signature: sig,
	}
}

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
