package lang

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strconv"
	"strings"

	"github.com/expr-lang/expr/vm"

	"github.com/ardnew/aenv/lang/lexer"
	"github.com/ardnew/aenv/lang/parser"
	"github.com/ardnew/aenv/lang/parser/bsr"
	"github.com/ardnew/aenv/lang/token"
	"github.com/ardnew/aenv/log"
)

// AST represents the abstract syntax tree for the aenv language.
type AST struct {
	Namespaces []*Namespace
	opts       optionsKey   // configuration options
	build      buildContext // build state (used during parsing)
	logger     log.Logger   // structured logger (outside optionsKey, doesn't affect cache)
}

// DefineNamespace adds a new namespace to the AST and returns it.
func (ast *AST) DefineNamespace(
	identifier string,
	params []*Value,
	value *Value,
) *Namespace {
	ns := &Namespace{
		Identifier: newToken("identifier", identifier),
		Parameters: params,
		Value:      value,
	}
	ast.Namespaces = append(ast.Namespaces, ns)

	return ns
}

// GetNamespace retrieves a namespace by its identifier.
// Returns (nil, false) if the namespace is not found.
func (ast *AST) GetNamespace(name string) (*Namespace, bool) {
	for _, ns := range ast.Namespaces {
		if ns.Identifier.LiteralString() == name {
			return ns, true
		}
	}

	return nil, false
}

// All returns an iterator over all namespaces in the AST.
func (ast *AST) All() iter.Seq[*Namespace] {
	return func(yield func(*Namespace) bool) {
		for _, ns := range ast.Namespaces {
			if !yield(ns) {
				return
			}
		}
	}
}

// Namespace represents a namespace declaration: identifier [Parameters] :
// Value.
type Namespace struct {
	Identifier *token.Token
	Parameters []*Value
	Value      *Value
}

// Tuple represents a tuple: { [Values] }.
type Tuple struct {
	Values []*Value
}

// Value represents any value expression in the language.
type Value struct {
	Type Type
	// Exactly one of these will be set based on Type
	Token     *token.Token // For identifiers, literals, keywords
	Tuple     *Tuple       // For tuples/values
	Namespace *Namespace   // For recursive namespaces
	Program   *vm.Program  // Compiled expr program (TypeExpr only)
}

// ExprSource returns the raw expression source for TypeExpr values.
// It strips the surrounding {{ and }} delimiters and trims whitespace.
func (v *Value) ExprSource() string {
	if v.Type != TypeExpr || v.Token == nil {
		return ""
	}

	s := v.Token.LiteralString()
	s = strings.TrimPrefix(s, "{{")
	s = strings.TrimSuffix(s, "}}")

	return strings.TrimSpace(s)
}

// Type indicates the type of value.
type Type int

const (
	// TypeIdentifier represents an identifier reference to another namespace.
	TypeIdentifier Type = iota

	// TypeBoolean represents a boolean literal value.
	TypeBoolean

	// TypeNumber represents a numeric literal value.
	TypeNumber

	// TypeString represents a string literal value.
	TypeString

	// TypeExpr represents an expression literal value.
	TypeExpr

	// TypeTuple represents a tuple of key-value pairs.
	TypeTuple

	// TypeNamespace represents a nested namespace.
	TypeNamespace
)

// String returns a string representation of the value type.
func (vt Type) String() string {
	switch vt {
	case TypeIdentifier:
		return "Identifier"

	case TypeBoolean:
		return "Boolean"

	case TypeNumber:
		return "Number"

	case TypeString:
		return "String"

	case TypeExpr:
		return "Expr"

	case TypeTuple:
		return "Tuple"

	case TypeNamespace:
		return "Namespace"

	default:
		return "Unknown"
	}
}

// DefaultMaxDepth is the default maximum depth for recursive namespaces.
// Users may modify this before parsing to change the default.
var DefaultMaxDepth = 100

// optionsKey holds AST configuration options.
// This type is gob-encodable for cache key hashing.
type optionsKey struct {
	maxDepth     int
	compileExprs bool
	processEnv   []string
}

// buildContext tracks state during AST construction.
type buildContext struct {
	depth int
	chain []string
}

// Option configures AST parsing or evaluation behavior.
type Option func(*AST)

// WithMaxDepth sets the maximum recursion depth for nested namespaces.
func WithMaxDepth(depth int) Option {
	return func(ast *AST) {
		ast.opts.maxDepth = depth
	}
}

// WithCompileExprs enables expression compilation during parsing.
func WithCompileExprs(compile bool) Option {
	return func(ast *AST) {
		ast.opts.compileExprs = compile
	}
}

// WithProcessEnv sets the environment variables for expression evaluation.
// The format is []string{"KEY=VALUE", ...}. If nil, os.Environ() is used.
func WithProcessEnv(env []string) Option {
	return func(ast *AST) {
		ast.opts.processEnv = env
	}
}

// WithLogger sets the structured logger for trace-level debugging.
// If not provided, the logger is zero-valued and all logging is a no-op.
func WithLogger(logger log.Logger) Option {
	return func(ast *AST) {
		ast.logger = logger
	}
}

// applyDefaults sets default option values on an AST.
func applyDefaults(ast *AST) {
	ast.opts.maxDepth = DefaultMaxDepth
}

// applyOptions applies functional options to an AST.
func applyOptions(ast *AST, opts ...Option) {
	for _, opt := range opts {
		opt(ast)
	}
}

// ParseString parses input string and returns the AST.
// Options can be provided to customize parsing behavior.
// The result is cached for efficient repeated parsing of the same content
// when no options or default options are used.
func ParseString(
	ctx context.Context,
	input string,
	opts ...Option,
) (*AST, error) {
	var tempAST AST

	applyDefaults(&tempAST)
	applyOptions(&tempAST, opts...)

	tempAST.logger.TraceContext(
		ctx,
		"parse start",
		slog.Int("source_length", len(input)),
	)

	if len(opts) == 0 {
		return parseStringCached(ctx, input)
	}

	ast, err := parse(ctx, lexer.New([]rune(input)), input, opts...)

	// If it's a ParseError, attach the source input for better error messages
	pe := &ParseError{}
	if errors.As(err, &pe) {
		pe.Source = input
	}

	return ast, err
}

// Parse parses the lexer output and returns the AST.
// Options can be provided to customize parsing behavior.
func Parse(ctx context.Context, l *lexer.Lexer, opts ...Option) (*AST, error) {
	return parse(ctx, l, "", opts...)
}

// ParseValue parses a single value expression from a string.
// The input can be any valid aenv value: literal, tuple, identifier, or
// definition. For expressions ({{ ... }}), they are parsed but not compiled.
func ParseValue(
	ctx context.Context,
	input string,
	opts ...Option,
) (*Value, error) {
	// Wrap in a dummy definition to reuse the full parser
	wrapped := "_: " + input

	ast, err := ParseString(ctx, wrapped, opts...)
	if err != nil {
		return nil, err
	}

	if len(ast.Namespaces) == 0 {
		return nil, ErrInvalidToken.With(slog.String("input", input))
	}

	return ast.Namespaces[0].Value, nil
}

// parse is the internal parsing implementation.
func parse(
	ctx context.Context,
	l *lexer.Lexer,
	source string,
	opts ...Option,
) (*AST, error) {
	var tempAST AST

	applyDefaults(&tempAST)
	applyOptions(&tempAST, opts...)

	tempAST.logger.TraceContext(ctx, "lexer created")

	bsrSet, errs := parser.Parse(l)

	tempAST.logger.TraceContext(
		ctx,
		"parser complete",
		slog.Int("error_count", len(errs)),
	)

	if len(errs) > 0 {
		return nil, NewParseError(errs, source)
	}

	ast, err := buildAST(ctx, bsrSet, source, opts...)
	if err != nil {
		return ast, err
	}

	ast.logger.TraceContext(
		ctx,
		"ast built",
		slog.Int("namespace_count", len(ast.Namespaces)),
	)

	err = ast.CompileExprs(ctx)
	if err != nil {
		return ast, err
	}

	ast.logger.TraceContext(ctx, "expressions compiled")

	return ast, nil
}

// formatAmbiguityError creates a formatted error for ambiguous parses with
// source snippet.
func formatAmbiguityError(source string, line, col int) error {
	lines := strings.Split(source, "\n")

	var buf strings.Builder

	// Write error location and description
	buf.WriteString(fmt.Sprintf("line %d, column %d:\n", line, col))

	// Show the offending line if within bounds
	if line > 0 && line <= len(lines) {
		lineIdx := line - 1
		lineText := lines[lineIdx]

		// Print the line with line number
		buf.WriteString(fmt.Sprintf("  %d | %s\n", line, lineText))

		// Print marker pointing to the column
		// Calculate the width needed for line number display
		lineNumWidth := len(strconv.Itoa(line))
		// +5 accounts for: 2 leading spaces + " | " (3 chars)
		padding := strings.Repeat(" ", lineNumWidth+5)

		// Add spaces to reach the error column
		if col > 0 {
			padding += strings.Repeat(" ", col-1)
		}

		buf.WriteString(padding + "^\n")
	}

	return ErrAmbiguousParse.Wrap(errors.New(buf.String()))
}

// buildAST constructs the AST from the BSR parse forest.
func buildAST(
	ctx context.Context,
	bsrSet *bsr.Set,
	source string,
	opts ...Option,
) (ast *AST, err error) {
	// Create AST and apply options
	ast = &AST{}
	applyDefaults(ast)
	applyOptions(ast, opts...)

	// Catch panics from ambiguous parses and convert to proper errors
	defer func() {
		if r := recover(); r != nil {
			panicMsg, ok := r.(string)
			if !ok {
				// Not a string panic, re-panic
				panic(r)
			}

			if !strings.Contains(panicMsg, "is ambiguous in") {
				// Not an ambiguity error, re-panic
				panic(r)
			}

			// Extract line and column from panic message
			// Format: "Error in BSR: ... at line N col M"
			var line, col int

			atIndex := strings.LastIndex(panicMsg, " at line ")
			if atIndex >= 0 {
				lineColPart := panicMsg[atIndex:]
				if _, scanErr := fmt.Sscanf(lineColPart, " at line %d col %d", &line, &col); scanErr == nil {
					// Format error with source snippet
					err = formatAmbiguityError(source, line, col)

					return
				}
			}

			// Couldn't parse line/col, re-panic
			panic(r)
		}
	}()

	// The root is Manifest : Namespaces
	root := bsrSet.GetRoot()

	ast.logger.TraceContext(ctx, "bsr root obtained")

	namespacesNode := root.GetNTChildI(0) // Get the Namespaces NT at position 0

	ns, err := ast.buildNamespaces(ctx, namespacesNode)
	if err != nil {
		return nil, err
	}

	ast.Namespaces = ns
	ast.resetBuildState()

	return ast, nil
}

// Print writes a formatted representation of the AST to the writer.
func (ast *AST) Print(ctx context.Context, w io.Writer) {
	ast.PrintIndent(ctx, w, 0)
}

// PrintIndent writes a formatted representation of the AST to the writer
// with the specified indentation.
func (ast *AST) PrintIndent(ctx context.Context, w io.Writer, indent int) {
	for _, ns := range ast.Namespaces {
		ns.Print(ctx, w, indent)
	}
}

// resetBuildState clears build context after parsing completes.
func (ast *AST) resetBuildState() {
	ast.build.depth = 0
	ast.build.chain = nil
}

// Namespaces : Namespace Namespaces | Namespace separator Namespaces |
// empty.
func (ast *AST) buildNamespaces(
	ctx context.Context,
	b bsr.BSR,
) ([]*Namespace, error) {
	alt := b.Alternate()

	if alt == 2 {
		// empty alternate
		return nil, nil
	}

	// Get the first namespace (always at index 0)
	ns, err := ast.buildNamespace(ctx, b.GetNTChildI(0))
	if err != nil {
		return nil, err
	}

	if alt == 0 {
		// Namespace Namespaces (no separator)
		// 0=Namespace 1=Namespaces
		rest, err := ast.buildNamespaces(ctx, b.GetNTChildI(1))
		if err != nil {
			return nil, err
		}

		return append([]*Namespace{ns}, rest...), nil
	}

	// alt == 1: Namespace separator Namespaces
	// 0=Namespace 1=separator 2=Namespaces
	rest, err := ast.buildNamespaces(ctx, b.GetNTChildI(2))
	if err != nil {
		return nil, err
	}

	return append([]*Namespace{ns}, rest...), nil
}

// Namespace : identifier Parameters op_define Value.
func (ast *AST) buildNamespace(
	ctx context.Context,
	b bsr.BSR,
) (*Namespace, error) {
	// Check recursion depth
	if ast.build.depth >= ast.opts.maxDepth {
		return nil, ErrMaxDepthExceeded.
			With(slog.Int("depth", ast.build.depth)).
			With(slog.Int("max_depth", ast.opts.maxDepth)).
			With(slog.String("chain", strings.Join(ast.build.chain, " → ")))
	}

	identToken := b.GetTChildI(0)
	identName := identToken.LiteralString()

	// Track this namespace in the chain
	ast.build.chain = append(ast.build.chain, identName)
	ast.build.depth++

	defer func() {
		ast.build.depth--
		ast.build.chain = ast.build.chain[:len(ast.build.chain)-1]
	}()

	// 0=identifier 1=Parameters 2=op_define 3=Value
	params, err := ast.buildParameters(ctx, b.GetNTChildI(1))
	if err != nil {
		return nil, err
	}

	ast.logger.TraceContext(
		ctx,
		"build namespace",
		slog.String("identifier", identName),
		slog.Int("depth", ast.build.depth),
		slog.Int("param_count", len(params)),
	)

	val, err := ast.buildValue(ctx, b.GetNTChildI(3))
	if err != nil {
		return nil, err
	}

	return &Namespace{
		Identifier: identToken,
		Parameters: params,
		Value:      val,
	}, nil
}

// Parameters : identifier Parameters | empty.
func (ast *AST) buildParameters(
	ctx context.Context,
	b bsr.BSR,
) ([]*Value, error) {
	if b.Alternate() == 1 {
		// empty alternate
		return nil, nil
	}

	// 0=identifier 1=Parameters
	tok := b.GetTChildI(0)
	val := &Value{Type: TypeIdentifier, Token: tok}

	rest, err := ast.buildParameters(ctx, b.GetNTChildI(1))
	if err != nil {
		return nil, err
	}

	return append([]*Value{val}, rest...), nil
}

// Tuple : "{" Values "}".
func (ast *AST) buildTuple(ctx context.Context, b bsr.BSR) (*Tuple, error) {
	// 0="{" 1=Values 2="}"
	values, err := ast.buildValues(ctx, b.GetNTChildI(1))
	if err != nil {
		return nil, err
	}

	return &Tuple{Values: values}, nil
}

// Values : Value | Value delimiter Values | empty.
func (ast *AST) buildValues(ctx context.Context, b bsr.BSR) ([]*Value, error) {
	switch b.Alternate() {
	case 0: // Value
		val, err := ast.buildValue(ctx, b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		return []*Value{val}, nil

	case 1: // 0=Value 1=delimiter 2=Values
		val, err := ast.buildValue(ctx, b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		rest, err := ast.buildValues(ctx, b.GetNTChildI(2))
		if err != nil {
			return nil, err
		}

		return append([]*Value{val}, rest...), nil

	case 2: // empty
		return nil, nil

	default:
		return nil, ErrInvalidToken.With(slog.String("token", "Values"))
	}
}

// Value : Literal | Tuple | Definition | identifier.
func (ast *AST) buildValue(ctx context.Context, b bsr.BSR) (*Value, error) {
	alt := b.Alternate()
	alternate := "Unknown"

	switch alt {
	case 0:
		alternate = "Literal"
	case 1:
		alternate = "Tuple"
	case 2:
		alternate = "Namespace"
	case 3:
		alternate = "Identifier"
	}

	ast.logger.TraceContext(
		ctx,
		"build value",
		slog.String("alternate", alternate),
	)

	switch alt {
	case 0: // Literal
		return ast.buildLiteral(ctx, b.GetNTChildI(0))

	case 1: // Tuple
		tuple, err := ast.buildTuple(ctx, b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		return &Value{Type: TypeTuple, Tuple: tuple}, nil

	case 2: // Namespace (recursive!)
		ns, err := ast.buildNamespace(ctx, b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		return &Value{Type: TypeNamespace, Namespace: ns}, nil

	case 3: // identifier
		return &Value{Type: TypeIdentifier, Token: b.GetTChildI(0)}, nil

	default:
		attr := []slog.Attr{slog.String("token", "Value")}

		return nil, ErrInvalidToken.With(attr...)
	}
}

// Literal : boolean_literal | number_literal | string_literal | expr_literal.
func (ast *AST) buildLiteral(ctx context.Context, b bsr.BSR) (*Value, error) {
	switch b.Alternate() {
	case 0: // boolean_literal
		tok := b.GetTChildI(0)
		ast.logger.TraceContext(
			ctx,
			"build literal",
			slog.String("type", "Boolean"),
			slog.String("value", tok.LiteralString()),
		)

		return &Value{Type: TypeBoolean, Token: tok}, nil

	case 1: // number_literal
		tok := b.GetTChildI(0)
		ast.logger.TraceContext(
			ctx,
			"build literal",
			slog.String("type", "Number"),
			slog.String("value", tok.LiteralString()),
		)

		return &Value{Type: TypeNumber, Token: tok}, nil

	case 2: // string_literal
		tok := b.GetTChildI(0)
		ast.logger.TraceContext(
			ctx,
			"build literal",
			slog.String("type", "String"),
			slog.String("value", tok.LiteralString()),
		)

		return &Value{Type: TypeString, Token: tok}, nil

	case 3: // expr_literal
		tok := b.GetTChildI(0)
		ast.logger.TraceContext(
			ctx,
			"build literal",
			slog.String("type", "Expr"),
			slog.String("value", tok.LiteralString()),
		)

		return &Value{Type: TypeExpr, Token: tok}, nil

	default:
		return nil, ErrInvalidToken.With(slog.String("token", "Literal"))
	}
}

func writer(w io.Writer) func(eol string, item ...string) {
	return func(eol string, item ...string) {
		_, err := io.WriteString(w, strings.Join(item, ": ")+eol)
		if err != nil {
			panic(err)
		}
	}
}

// Print writes a formatted representation of the namespace.
func (ns *Namespace) Print(ctx context.Context, w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)
	put := writer(w)
	put("\n", prefix+"Namespace", ns.Identifier.LiteralString())

	if len(ns.Parameters) > 0 {
		put(":\n", prefix+"  Parameters")

		for _, param := range ns.Parameters {
			param.Print(ctx, w, indent+2)
		}
	}

	put(":\n", prefix+"  Value")
	ns.Value.Print(ctx, w, indent+2)
}

// Print writes a formatted representation of the tuple.
func (t *Tuple) Print(ctx context.Context, w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)

	if len(t.Values) == 0 {
		writer(w)("\n", prefix+"(empty)")

		return
	}

	for _, val := range t.Values {
		val.Print(ctx, w, indent)
	}
}

// Print writes a formatted representation of the value.
func (v *Value) Print(ctx context.Context, w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)
	put := writer(w)

	switch v.Type {
	case TypeTuple:
		put("", prefix+"Tuple", "")
		put("\n")
		v.Tuple.Print(ctx, w, indent+1)

	case TypeNamespace:
		// For nested namespaces, print them directly without the "Namespace:"
		// prefix
		// since Namespace.Print already adds that
		v.Namespace.Print(ctx, w, indent)

	default:
		put("", prefix+v.Type.String(), "")

		if v.Token != nil {
			put("\n", v.Token.LiteralString())
		} else {
			put("\n", "(nil)")
		}
	}
}
