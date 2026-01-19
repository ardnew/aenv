package lang

//go:generate ./internal/bootstrap.sh ${GOPACKAGE} ${GOFILE}

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/ardnew/envcomp/pkg"
	"github.com/ardnew/envcomp/pkg/lang/internal/lexer"
	"github.com/ardnew/envcomp/pkg/lang/internal/parser"
	"github.com/ardnew/envcomp/pkg/lang/internal/parser/bsr"
	"github.com/ardnew/envcomp/pkg/lang/internal/token"
)

// AST represents the abstract syntax tree for the envcomp language.
type AST struct {
	Definitions []*Definition
}

// Definition represents a definition declaration: identity [Parameters] :
// Value.
type Definition struct {
	Identifier *token.Token
	Parameters []*Value
	Value      *Value
}

// Tuple represents a tuple: { [Aggregate] }.
type Tuple struct {
	Aggregate []*Value
}

// Value represents any value expression in the language.
type Value struct {
	Type Type
	// Exactly one of these will be set based on Type
	Token      *token.Token // For identifiers, literals, keywords
	Tuple      *Tuple       // For tuples/aggregates
	Definition *Definition  // For recursive definitions
	Template   *Template    // For template expressions (deprecated)
}

// Template represents a format string with arguments.
type Template struct {
	Format    *token.Token // The string literal format
	Arguments []*Value     // The list of arguments
}

// Type indicates the type of value.
type Type int

const (
	// TypeIdentifier represents an identifier reference to another definition.
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

	// TypeDefinition represents a nested definition.
	TypeDefinition
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
	case TypeDefinition:
		return "Definition"
	default:
		return "Unknown"
	}
}

// DefaultMaxDefinitionDepth is the default maximum depth for recursive
// definitions.
const DefaultMaxDefinitionDepth = 100

// ParseOptions configures the parser behavior.
type ParseOptions struct {
	MaxDefinitionDepth int
}

// DefaultParseOptions returns the default parse options.
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		MaxDefinitionDepth: DefaultMaxDefinitionDepth,
	}
}

// ParseString parses input string and returns the AST.
func ParseString(input string) (*AST, error) {
	return ParseStringWithOptions(input, DefaultParseOptions())
}

// ParseStringWithOptions parses input string with custom options.
func ParseStringWithOptions(input string, opts ParseOptions) (*AST, error) {
	ast, err := ParseWithOptions(lexer.New([]rune(input)), opts, input)
	if err != nil {
		// If it's a ParseError, attach the source input for better error messages
		pe := &ParseError{}
		if errors.As(err, &pe) {
			pe.Source = input
		}
	}

	return ast, err
}

// Parse parses the lexer output and returns the AST.
func Parse(l *lexer.Lexer) (*AST, error) {
	return ParseWithOptions(l, DefaultParseOptions(), "")
}

// ParseWithOptions parses the lexer output with custom options.
func ParseWithOptions(
	l *lexer.Lexer,
	opts ParseOptions,
	source string,
) (*AST, error) {
	bsrSet, errs := parser.Parse(l)
	if len(errs) > 0 {
		return nil, &ParseError{Errors: errs}
	}

	if bsrSet == nil {
		return nil, pkg.ErrNoParseTree
	}

	return buildAST(bsrSet, opts, source)
}

// ParseError wraps parser errors.
type ParseError struct {
	Errors   []*parser.Error
	Source   string   // The original source input
	Snippet  string   // Optional snippet of the source
	Expected []string // Optional expected tokens
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if len(e.Errors) == 0 {
		return "parse error"
	}

	// If we have the source, format with context
	if e.Source != "" {
		msg, snippet, expected := e.formatWithContext()
		e.Snippet = snippet
		e.Expected = expected

		return msg + snippet + "\texpected: " + strings.Join(expected, ", ")
	}

	return e.Errors[0].String()
}

// formatWithContext formats the parse error with source code context.
func (e *ParseError) formatWithContext() (string, string, []string) {
	if len(e.Errors) == 0 {
		return "parse error", "", nil
	}

	firstErr := e.Errors[0]
	lines := strings.Split(e.Source, "\n")

	var buf, src strings.Builder

	// Write error location and description
	buf.WriteString(fmt.Sprintf("parse error at line %d, column %d:\n",
		firstErr.Line, firstErr.Column))

	// Show the offending line if within bounds
	if firstErr.Line > 0 && firstErr.Line <= len(lines) {
		lineIdx := firstErr.Line - 1
		line := lines[lineIdx]

		// Print the line with line number
		src.WriteString(fmt.Sprintf("  %d | %s\n", firstErr.Line, line))

		// Print marker pointing to the column
		// Calculate the width needed for line number display
		lineNumWidth := len(strconv.Itoa(firstErr.Line))
		// +5 accounts for: 2 leading spaces + " | " (3 chars)
		padding := strings.Repeat(" ", lineNumWidth+5)

		// Add spaces to reach the error column
		if firstErr.Column > 0 {
			padding += strings.Repeat(" ", firstErr.Column-1)
		}

		src.WriteString(padding + "^\n")
	}

	// Write what was expected
	exp := []string{}
	for _, e := range firstErr.Expected {
		exp = append(exp, strconv.Quote(e))
	}

	slices.Sort(exp)

	return buf.String(), src.String(), exp
}

// formatAmbiguityError creates a formatted error for ambiguous parses with
// source snippet.
func formatAmbiguityError(source string, line, col int) error {
	lines := strings.Split(source, "\n")

	var buf strings.Builder

	// Write error location and description
	buf.WriteString(fmt.Sprintf("ambiguous parse at line %d, column %d:\n",
		line, col))

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

	buf.WriteString("\t")
	buf.WriteString(pkg.ErrAmbiguousParse.Error())

	return errors.New(buf.String())
}

// buildAST constructs the AST from the BSR parse forest.
func buildAST(
	bsrSet *bsr.Set,
	opts ParseOptions,
	source string,
) (ast *AST, err error) {
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

	// The root is Manifest : Definitions
	root := bsrSet.GetRoot()
	definitionsNode := root.GetNTChildI(0) // Get the Definitions NT at position 0

	ctx := &buildContext{
		opts:  opts,
		depth: 0,
		chain: []string{},
	}

	definitions, err := ctx.buildDefinitions(definitionsNode)
	if err != nil {
		return nil, err
	}

	return &AST{Definitions: definitions}, nil
}

// buildContext tracks state during AST construction.
type buildContext struct {
	opts  ParseOptions
	depth int
	chain []string
}

// Definitions : Definition Definitions | Definition separator Definitions |
// empty.
func (ctx *buildContext) buildDefinitions(b bsr.BSR) ([]*Definition, error) {
	alt := b.Alternate()

	if alt == 2 {
		// empty alternate
		return nil, nil
	}

	// Get the first definition (always at index 0)
	def, err := ctx.buildDefinition(b.GetNTChildI(0))
	if err != nil {
		return nil, err
	}

	if alt == 0 {
		// Definition Definitions (no separator)
		// 0=Definition 1=Definitions
		rest, err := ctx.buildDefinitions(b.GetNTChildI(1))
		if err != nil {
			return nil, err
		}

		return append([]*Definition{def}, rest...), nil
	}

	// alt == 1: Definition separator Definitions
	// 0=Definition 1=separator 2=Definitions
	rest, err := ctx.buildDefinitions(b.GetNTChildI(2))
	if err != nil {
		return nil, err
	}

	return append([]*Definition{def}, rest...), nil
}

// Definition : identifier Parameters op_define Value.
func (ctx *buildContext) buildDefinition(b bsr.BSR) (*Definition, error) {
	// Check recursion depth
	if ctx.depth >= ctx.opts.MaxDefinitionDepth {
		return nil, fmt.Errorf(
			"%w (%d); possible infinite recursion in chain: %s",
			pkg.ErrMaxDepthExceeded,
			ctx.opts.MaxDefinitionDepth,
			strings.Join(ctx.chain, " → "),
		)
	}

	identToken := b.GetTChildI(0)
	identName := identToken.LiteralString()

	// Track this definition in the chain
	ctx.chain = append(ctx.chain, identName)
	ctx.depth++

	defer func() {
		ctx.depth--
		ctx.chain = ctx.chain[:len(ctx.chain)-1]
	}()

	// 0=identifier 1=Parameters 2=op_define 3=Value
	params, err := ctx.buildParameters(b.GetNTChildI(1))
	if err != nil {
		return nil, err
	}

	val, err := ctx.buildValue(b.GetNTChildI(3))
	if err != nil {
		return nil, err
	}

	return &Definition{
		Identifier: identToken,
		Parameters: params,
		Value:      val,
	}, nil
}

// Parameters : Value Parameters | empty.
func (ctx *buildContext) buildParameters(b bsr.BSR) ([]*Value, error) {
	if b.Alternate() == 1 {
		// empty alternate
		return nil, nil
	}

	// Value Parameters
	val, err := ctx.buildValue(b.GetNTChildI(0))
	if err != nil {
		return nil, err
	}

	rest, err := ctx.buildParameters(b.GetNTChildI(1))
	if err != nil {
		return nil, err
	}

	return append([]*Value{val}, rest...), nil
}

// Tuple : "{" Aggregate "}".
func (ctx *buildContext) buildTuple(b bsr.BSR) (*Tuple, error) {
	// 0="{" 1=Aggregate 2="}"
	aggregate, err := ctx.buildAggregate(b.GetNTChildI(1))
	if err != nil {
		return nil, err
	}

	return &Tuple{Aggregate: aggregate}, nil
}

// Aggregate : Value | Value delimiter Aggregate | empty.
func (ctx *buildContext) buildAggregate(b bsr.BSR) ([]*Value, error) {
	switch b.Alternate() {
	case 0: // Value
		val, err := ctx.buildValue(b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		return []*Value{val}, nil

	case 1: // 0=Value 1=delimiter 2=Aggregate
		val, err := ctx.buildValue(b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		rest, err := ctx.buildAggregate(b.GetNTChildI(2))
		if err != nil {
			return nil, err
		}

		return append([]*Value{val}, rest...), nil
	case 2: // empty
		return nil, nil

	default:
		return nil, fmt.Errorf(
			"%w: Aggregate alternate %d",
			pkg.ErrInvalidAlternate,
			b.Alternate(),
		)
	}
}

// Value : Literal | Tuple | Definition | identifier.
func (ctx *buildContext) buildValue(b bsr.BSR) (*Value, error) {
	switch b.Alternate() {
	case 0: // Literal
		return ctx.buildLiteral(b.GetNTChildI(0))

	case 1: // Tuple
		tuple, err := ctx.buildTuple(b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		return &Value{Type: TypeTuple, Tuple: tuple}, nil

	case 2: // Definition (recursive!)
		def, err := ctx.buildDefinition(b.GetNTChildI(0))
		if err != nil {
			return nil, err
		}

		return &Value{Type: TypeDefinition, Definition: def}, nil

	case 3: // identifier
		return &Value{Type: TypeIdentifier, Token: b.GetTChildI(0)}, nil

	default:
		return nil, fmt.Errorf(
			"%w: Value alternate %d",
			pkg.ErrInvalidAlternate,
			b.Alternate(),
		)
	}
}

// Literal : boolean_literal | number_literal | string_literal | expr_literal.
func (ctx *buildContext) buildLiteral(b bsr.BSR) (*Value, error) {
	switch b.Alternate() {
	case 0: // boolean_literal
		return &Value{Type: TypeBoolean, Token: b.GetTChildI(0)}, nil

	case 1: // number_literal
		return &Value{Type: TypeNumber, Token: b.GetTChildI(0)}, nil

	case 2: // string_literal
		return &Value{Type: TypeString, Token: b.GetTChildI(0)}, nil

	case 3: // expr_literal
		return &Value{Type: TypeExpr, Token: b.GetTChildI(0)}, nil

	default:
		return nil, fmt.Errorf(
			"%w: Literal alternate %d",
			pkg.ErrInvalidAlternate,
			b.Alternate(),
		)
	}
}

// Print writes a formatted representation of the AST to the writer.
func (ast *AST) Print(w io.Writer) {
	ast.PrintIndent(w, 0)
}

// PrintIndent writes a formatted representation of the AST to the writer
// with the specified indentation.
func (ast *AST) PrintIndent(w io.Writer, indent int) {
	for _, def := range ast.Definitions {
		def.Print(w, indent)
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

// Print writes a formatted representation of the definition.
func (def *Definition) Print(w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)
	put := writer(w)
	put("\n", prefix+"Definition", def.Identifier.LiteralString())

	if len(def.Parameters) > 0 {
		put(":\n", prefix+"  Parameters")

		for _, param := range def.Parameters {
			param.Print(w, indent+2)
		}
	}

	put(":\n", prefix+"  Value")
	def.Value.Print(w, indent+2)
}

// Print writes a formatted representation of the tuple.
func (t *Tuple) Print(w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)

	if len(t.Aggregate) == 0 {
		writer(w)("\n", prefix+"(empty)")

		return
	}

	for _, val := range t.Aggregate {
		val.Print(w, indent)
	}
}

// Print writes a formatted representation of the value.
func (v *Value) Print(w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)
	put := writer(w)

	switch v.Type {
	case TypeTuple:
		put("", prefix+"Tuple", "")
		put("\n")
		v.Tuple.Print(w, indent+1)

	case TypeDefinition:
		// For nested definitions, print them directly without the "Definition:"
		// prefix
		// since Definition.Print already adds that
		v.Definition.Print(w, indent)

	default:
		put("", prefix+v.Type.String(), "")

		if v.Token != nil {
			put("\n", v.Token.LiteralString())
		} else {
			put("\n", "(nil)")
		}
	}
}
