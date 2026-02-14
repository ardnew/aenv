// Package lang provides a simplified DSL for generating environment variable
// definitions. It is a redesign of the lang package that eliminates complexity
// by delegating all expression parsing, compilation, and evaluation to
// expr-lang.
//
// # Philosophy
//
// Every value in lang2 is an expr-lang expression. The grammar has only two
// value kinds:
//
//   - Expression: raw text passed to expr-lang (e.g., 42, "hello", x + 1)
//   - Block: { namespace; ... } grouping named entries into a map
//
// No parser generator. No generated code. No double-brace syntax. No separate
// compile phase. The grammar is simple enough for a hand-written recursive
// descent parser, and expr-lang handles all expression semantics.
//
// # Grammar
//
// Informal EBNF:
//
//	Manifest    → Namespace* EOF
//	Namespace   → Identifier Param* ':' Value
//	Param       → Identifier | '...' Identifier
//	Value       → Block | Expression
//	Block       → '{' (Namespace (Sep Namespace)* Sep?)? '}'
//	Sep         → ';'
//	Expression  → <balanced text, stops at '}', ';', or EOF>
//
// # Example
//
//	# Simple values (expr-lang expressions)
//	port : 8080
//	greeting : "Hello, World!"
//	verbose : true
//
//	# Namespace references (resolved via patching)
//	server-port : port + 1000
//
//	# Parameterized namespaces (become expr-lang functions)
//	greet name : "Hello, " + name + "!"
//	add a b : a + b
//
//	# Blocks (evaluate to map[string]any)
//	config : {
//	  host : "localhost"
//	  port : 8080
//	  url : "http://" + host + ":" + string(port)
//	}
//
//	# Arrays use expr-lang syntax
//	items : ["apple", "banana", "cherry"]
//
// # Scoping
//
// Scoping is hierarchical, innermost shadows outermost:
//
//  1. Builtins (target, platform, cwd, file.*, path.*, mung.*)
//  2. Top-level namespaces (from AST root)
//  3. Block namespaces (from enclosing block, if any)
//  4. Parameters (from function invocation)
//
// Non-parameterized namespaces are patched as constants via expr.Patch.
// Parameterized namespaces are registered as callable functions via
// expr.Function.
package lang
