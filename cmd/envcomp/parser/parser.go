package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/ardnew/envcomp/pkg"
	"github.com/ardnew/envcomp/pkg/lang"
)

var (
	// globalCache stores definitions keyed by (source_hash:identifier).
	// This allows efficient lookup without keeping full ASTs in memory.
	globalCache sync.Map

	// globalRegistry tracks source metadata by source hash.
	globalRegistry sync.Map
)

// state tracks parsing state and top-level definition list for a source.
type state struct {
	once        sync.Once
	identifiers []string // List of definition identifiers found
	err         error
}

// Parser provides streaming access to definitions in envcomp source text.
// It parses on-demand and caches individual definitions, not full ASTs.
type Parser struct {
	reader    io.Reader
	source    string
	sourceKey string
	metadata  *state
}

// New creates a streaming parser from an io.Reader.
// The reader will not be consumed until first definition access.
func New(r io.Reader) *Parser {
	var p Parser

	p.reader = r
	p.metadata = new(state)

	return &p
}

// NewFromString creates a streaming parser from a source string.
func NewFromString(source string) *Parser {
	// Create source key (hash) for caching
	hash := sha256.Sum256([]byte(source))
	sourceKey := hex.EncodeToString(hash[:])

	// Get or create metadata entry
	entry := new(state)
	value, _ := globalRegistry.LoadOrStore(sourceKey, entry)
	metadata := value.(*state)

	return &Parser{
		source:    source,
		sourceKey: sourceKey,
		metadata:  metadata,
	}
}

// ensureParsed ensures the source has been read and parsed.
// This extracts and caches individual namespaces on first access.
func (p *Parser) ensureParsed() error {
	p.metadata.once.Do(func() {
		// Read source if from reader
		if p.source == "" && p.reader != nil {
			data, err := io.ReadAll(p.reader)
			if err != nil {
				p.metadata.err = fmt.Errorf("%w: %w", pkg.ErrReadInput, err)

				return
			}

			p.source = string(data)

			// Generate source key
			hash := sha256.Sum256(data)
			p.sourceKey = hex.EncodeToString(hash[:])
		}

		// Parse source to extract definitions
		ast, err := lang.ParseString(p.source)
		if err != nil {
			p.metadata.err = fmt.Errorf("%w: %w", pkg.ErrParse, err)

			return
		}

		// Cache each definition individually and track identifiers
		p.metadata.identifiers = make([]string, len(ast.Definitions))
		for i, def := range ast.Definitions {
			id := def.Identifier.LiteralString()
			p.metadata.identifiers[i] = id
			cacheKey := p.sourceKey + ":" + id
			globalCache.Store(cacheKey, def)
		}
	})

	return p.metadata.err
}

// GetDefinition retrieves a definition by its identifier.
// Returns an error if parsing fails or the definition is not found.
func (p *Parser) GetDefinition(name string) (*lang.Definition, error) {
	err := p.ensureParsed()
	if err != nil {
		return nil, err
	}

	cacheKey := p.sourceKey + ":" + name
	if value, ok := globalCache.Load(cacheKey); ok {
		return value.(*lang.Definition), nil
	}

	return nil, fmt.Errorf("%w: %q", pkg.ErrDefinitionNotFound, name)
}

// Definitions returns an iterator over all definitions in the source.
// If parsing fails, the iterator yields no values.
func (p *Parser) Definitions() iter.Seq[*lang.Definition] {
	return func(yield func(*lang.Definition) bool) {
		err := p.ensureParsed()
		if err != nil {
			return
		}

		for _, id := range p.metadata.identifiers {
			cacheKey := p.sourceKey + ":" + id
			if value, ok := globalCache.Load(cacheKey); ok {
				if !yield(value.(*lang.Definition)) {
					return
				}
			}
		}
	}
}

// AST returns the complete parsed AST.
// This reconstructs the AST from cached definitions.
// Use sparingly - prefer GetDefinition or Definitions for efficiency.
func (p *Parser) AST() (*lang.AST, error) {
	err := p.ensureParsed()
	if err != nil {
		return nil, err
	}

	ast := &lang.AST{
		Definitions: make([]*lang.Definition, len(p.metadata.identifiers)),
	}

	for i, id := range p.metadata.identifiers {
		cacheKey := p.sourceKey + ":" + id
		if value, ok := globalCache.Load(cacheKey); ok {
			ast.Definitions[i] = value.(*lang.Definition)
		}
	}

	return ast, nil
}

// Functional-style interfaces for direct use without creating a Parser
// instance.

// GetDefinitionFrom retrieves a definition by identifier from an io.Reader.
// This is a convenience function that creates a parser and retrieves the
// definition.
func GetDefinitionFrom(r io.Reader, name string) (*lang.Definition, error) {
	return New(r).GetDefinition(name)
}

// DefinitionsFrom returns an iterator over all definitions from an io.Reader.
// This is a convenience function that creates a parser and returns the
// iterator.
func DefinitionsFrom(r io.Reader) iter.Seq[*lang.Definition] {
	return New(r).Definitions()
}

// ClearCache removes all cached definitions and source metadata.
// This is primarily useful for testing or when memory needs to be reclaimed.
func ClearCache() {
	globalCache = sync.Map{}
	globalRegistry = sync.Map{}
}
