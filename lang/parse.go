package lang

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ardnew/aenv/log"
)

// ParseReader parses an AST from an io.Reader.
func ParseReader(
	ctx context.Context,
	r io.Reader,
	opts ...Option,
) (*AST, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, WrapError(err)
	}

	return ParseString(ctx, string(data), opts...)
}

// ParseString parses an AST from a string.
func ParseString(ctx context.Context, s string, opts ...Option) (*AST, error) {
	var logger log.Logger

	p := &parser{
		input:  []byte(s),
		pos:    0,
		line:   1,
		col:    1,
		logger: logger,
	}

	ast, err := p.parseManifest()
	if err != nil {
		return nil, err
	}

	// Apply options
	for _, opt := range opts {
		opt(ast)
	}

	p.logger = ast.logger

	// Build namespace index for O(1) lookups
	ast.buildIndex()

	p.logger.TraceContext(ctx, "parse complete",
		slog.Int("namespace_count", len(ast.Namespaces)))

	return ast, nil
}

// parser holds the parser state.
type parser struct {
	input  []byte
	pos    int
	line   int
	col    int
	logger log.Logger
}

// parseManifest parses the entire input as a list of namespaces.
func (p *parser) parseManifest() (*AST, error) {
	ast := new(AST)
	ast.Namespaces = make([]*Namespace, 0)

	for {
		p.skipWhitespaceAndComments()

		if p.eof() {
			break
		}

		ns, err := p.parseNamespace()
		if err != nil {
			return nil, err
		}

		ast.Namespaces = append(ast.Namespaces, ns)

		// After a namespace, expect separator or EOF
		p.skipWhitespaceAndComments()

		if p.eof() {
			break
		}

		// Check for separator (optional)
		if p.peek() == ';' {
			p.advance()
			p.skipWhitespaceAndComments()
		}
	}

	return ast, nil
}

// parseNamespace parses: Identifier Param* ':' Value.
func (p *parser) parseNamespace() (*Namespace, error) {
	pos := p.position()

	name, err := p.parseIdentifier()
	if err != nil {
		return nil, ErrParse.WithPosition(pos).Wrap(err)
	}

	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}

	p.skipWhitespaceAndComments()

	if !p.expect(':') {
		return nil, ErrParse.WithPosition(p.position()).
			With(slog.String("expected", ":")).
			With(slog.String("name", name))
	}

	p.skipWhitespaceAndComments()

	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &Namespace{
		Name:   name,
		Params: params,
		Value:  value,
		Pos:    pos,
	}, nil
}

// parseParams parses zero or more parameters.
func (p *parser) parseParams() ([]Param, error) {
	params := make([]Param, 0)

	for {
		p.skipWhitespaceAndComments()

		if p.eof() || p.peek() == ':' {
			break
		}

		// Check for variadic prefix
		variadic := false
		if p.peek() == '.' && p.peekN(3) == "..." {
			variadic = true
			p.pos += 3
			p.col += 3
			p.skipWhitespaceAndComments()
		}

		// Must be followed by identifier
		if !isIdentifierStart(p.peek()) {
			break
		}

		name, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		params = append(params, Param{
			Name:     name,
			Variadic: variadic,
		})

		// Variadic must be last
		if variadic {
			break
		}
	}

	return params, nil
}

// parseValue parses a value (either Block or Expression).
func (p *parser) parseValue() (*Value, error) {
	pos := p.position()

	if p.peek() == '{' {
		// Disambiguate: is this a block or an expr-lang map literal?
		if p.isBlock() {
			return p.parseBlock()
		}
	}

	// Otherwise, capture as expression
	source, err := p.captureExpression()
	if err != nil {
		return nil, err
	}

	v := new(Value)
	v.Kind = KindExpr
	v.Source = source
	v.Pos = pos

	return v, nil
}

// isBlock checks if '{' starts a block (namespace group) or an expression.
func (p *parser) isBlock() bool {
	// Save position
	saved := p.pos

	p.advance() // skip '{'
	p.skipWhitespaceAndComments()

	// Empty block
	if p.peek() == '}' {
		p.pos = saved

		return true
	}

	// Check if it looks like "identifier ... :"
	if isIdentifierStart(p.peek()) {
		// Try to scan ahead for ':'
		tempPos := p.pos
		for tempPos < len(p.input) {
			ch := rune(p.input[tempPos])
			if ch == ':' {
				p.pos = saved

				return true
			}

			if ch == '{' || ch == '}' || ch == ';' {
				break
			}

			tempPos++
		}
	}

	p.pos = saved

	return false
}

// parseBlock parses: '{' (Namespace (Sep Namespace)* Sep?)? '}'.
func (p *parser) parseBlock() (*Value, error) {
	pos := p.position()

	if !p.expect('{') {
		return nil, ErrParse.WithPosition(pos).
			With(slog.String("expected", "{"))
	}

	entries := make([]*Namespace, 0)

	for {
		p.skipWhitespaceAndComments()

		if p.eof() {
			return nil, ErrParse.WithPosition(p.position()).
				With(slog.String("expected", "}"))
		}

		if p.peek() == '}' {
			p.advance()

			break
		}

		ns, err := p.parseNamespace()
		if err != nil {
			return nil, err
		}

		entries = append(entries, ns)

		// Check for separator
		p.skipWhitespaceAndComments()

		if p.peek() == ';' {
			p.advance()
			p.skipWhitespaceAndComments()
		}
	}

	v := new(Value)
	v.Kind = KindBlock
	v.Entries = entries
	v.Pos = pos

	return v, nil
}

// captureExpression captures raw expression text.
// Stops at unbalanced '}', ',', ';', or EOF.
// Tracks balanced '()', '[]', '{}'.
// Skips string literals so delimiters inside strings don't terminate.
func (p *parser) captureExpression() (string, error) {
	start := p.pos
	depth := 0 // track nesting of (), [], {}

	for p.pos < len(p.input) {
		ch := p.peek()

		// Skip string literals
		if ch == '"' || ch == '\'' || ch == '`' {
			err := p.skipString(ch)
			if err != nil {
				return "", err
			}

			continue
		}

		// Skip comments
		if ch == '/' && p.peekN(2) == "//" {
			p.skipLineComment()

			continue
		}

		if ch == '/' && p.peekN(2) == "/*" {
			p.skipBlockComment()

			continue
		}

		if ch == '#' {
			p.skipLineComment()

			continue
		}

		// Track depth
		switch ch {
		case '(', '[', '{':
			depth++

			p.advance()
		case ')', ']', '}':
			if depth == 0 {
				// Unbalanced closing - stop here
				goto done
			}

			depth--

			p.advance()
		case ';':
			if depth == 0 {
				// Top-level separator - stop here
				goto done
			}

			p.advance()
		default:
			p.advance()
		}
	}

done:
	source := string(p.input[start:p.pos])
	// Trim whitespace and strip comments
	source = stripComments(source)

	return source, nil
}

// parseIdentifier parses an identifier token.
func (p *parser) parseIdentifier() (string, error) {
	start := p.pos

	if !isIdentifierStart(p.peek()) {
		return "", ErrParse.WithPosition(p.position()).
			With(slog.String("expected", "identifier"))
	}

	p.advance()

	// Continue with identifier chars
	for !p.eof() && isIdentifierContinue(p.peek()) {
		p.advance()
	}

	// Handle internal separators (-, +, ., @, /)
	for !p.eof() {
		ch := p.peek()
		if ch == '-' || ch == '+' || ch == '.' || ch == '@' || ch == '/' {
			// Look ahead - must be followed by identifier char
			if p.pos+1 < len(p.input) &&
				isIdentifierContinue(rune(p.input[p.pos+1])) {
				p.advance() // skip separator
				// Continue with identifier chars
				for !p.eof() && isIdentifierContinue(p.peek()) {
					p.advance()
				}
			} else {
				break
			}
		} else {
			break
		}
	}

	return string(p.input[start:p.pos]), nil
}

// Helper methods

func (p *parser) peek() rune {
	if p.eof() {
		return 0
	}

	r, _ := utf8.DecodeRune(p.input[p.pos:])

	return r
}

func (p *parser) peekN(n int) string {
	if p.pos+n > len(p.input) {
		return string(p.input[p.pos:])
	}

	return string(p.input[p.pos : p.pos+n])
}

func (p *parser) advance() {
	if p.eof() {
		return
	}

	r, size := utf8.DecodeRune(p.input[p.pos:])

	p.pos += size
	if r == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
}

func (p *parser) expect(ch rune) bool {
	if p.peek() == ch {
		p.advance()

		return true
	}

	return false
}

func (p *parser) eof() bool {
	return p.pos >= len(p.input)
}

func (p *parser) position() Position {
	return Position{
		Offset: p.pos,
		Line:   p.line,
		Column: p.col,
	}
}

func (p *parser) skipWhitespace() {
	for !p.eof() && unicode.IsSpace(p.peek()) {
		p.advance()
	}
}

func (p *parser) skipWhitespaceAndComments() {
	for {
		p.skipWhitespace()

		if p.eof() {
			return
		}

		// Line comment
		if p.peek() == '/' && p.peekN(2) == "//" {
			p.skipLineComment()

			continue
		}

		if p.peek() == '#' {
			p.skipLineComment()

			continue
		}

		// Block comment
		if p.peek() == '/' && p.peekN(2) == "/*" {
			p.skipBlockComment()

			continue
		}

		break
	}
}

func (p *parser) skipLineComment() {
	for !p.eof() && p.peek() != '\n' {
		p.advance()
	}

	if !p.eof() {
		p.advance() // skip '\n'
	}
}

func (p *parser) skipBlockComment() {
	p.advance() // skip '/'
	p.advance() // skip '*'

	for !p.eof() {
		if p.peek() == '*' && p.peekN(2) == "*/" {
			p.advance() // skip '*'
			p.advance() // skip '/'

			return
		}

		p.advance()
	}
}

func (p *parser) skipString(quote rune) error {
	p.advance() // skip opening quote

	for !p.eof() {
		ch := p.peek()
		if ch == '\\' {
			p.advance() // skip backslash

			if !p.eof() {
				p.advance() // skip escaped char
			}

			continue
		}

		if ch == quote {
			p.advance() // skip closing quote

			return nil
		}

		p.advance()
	}

	return ErrParse.WithPosition(p.position()).
		With(slog.String("error", "unterminated string"))
}

// Character classification

func isIdentifierStart(r rune) bool {
	return unicode.In(r,
		unicode.L,  // Letter
		unicode.Nl, // Letter, Number
		unicode.Other_ID_Start,
	) || r == '_'
}

func isIdentifierContinue(r rune) bool {
	return unicode.In(r,
		unicode.L,  // Letter
		unicode.Nl, // Letter, Number
		unicode.Other_ID_Start,
		unicode.Mn, // Mark, Nonspacing
		unicode.Mc, // Mark, Spacing Combining
		unicode.Nd, // Number, Decimal Digit
		unicode.Pc, // Punctuation, Connector
		unicode.Other_ID_Continue,
	)
}

// stripComments removes comments from source text and trims whitespace.
func stripComments(s string) string {
	// Simple implementation - just trim for now
	// Could be more sophisticated to actually strip comments
	result := ""
	inString := false

	var stringChar rune

	i := 0

	var resultSb534 strings.Builder

	for i < len(s) {
		ch := rune(s[i])

		// Handle strings
		if !inString && (ch == '"' || ch == '\'' || ch == '`') {
			inString = true
			stringChar = ch
			resultSb534.WriteRune(ch)

			i++

			continue
		}

		if inString && ch == stringChar {
			inString = false

			resultSb534.WriteRune(ch)

			i++

			continue
		}

		if inString {
			resultSb534.WriteRune(ch)

			i++

			continue
		}

		// Skip line comments
		if ch == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}

			continue
		}

		if ch == '#' {
			for i < len(s) && s[i] != '\n' {
				i++
			}

			continue
		}

		// Skip block comments
		if ch == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i < len(s) {
				if s[i] == '*' && i+1 < len(s) && s[i+1] == '/' {
					i += 2

					break
				}

				i++
			}

			continue
		}

		resultSb534.WriteRune(ch)

		i++
	}

	result += resultSb534.String()

	// Trim whitespace
	return strings.TrimSpace(result)
}
