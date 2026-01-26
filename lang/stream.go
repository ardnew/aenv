package lang

import (
	"io"
	"iter"
	"log/slog"
	"strconv"
	"sync"

	"github.com/klauspost/readahead"
	"github.com/zeebo/xxh3"
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

// Stream provides streaming access to definitions in aenv source text.
// It parses on-demand and caches individual definitions, not full ASTs.
type Stream struct {
	reader    io.Reader
	source    string
	sourceKey string
	metadata  *state
}

// NewStream creates a streaming parser from an io.Reader.
// The reader will not be consumed until first definition access.
func NewStream(r io.Reader) *Stream {
	var p Stream

	p.reader = r
	p.metadata = new(state)

	return &p
}

// NewStreamFromString creates a streaming parser from a source string.
func NewStreamFromString(source string) *Stream {
	// Create source key (hash) for caching - using xxhash3 for performance
	hash := xxh3.Hash([]byte(source))
	sourceKey := strconv.FormatUint(hash, 36)

	// Get or create metadata entry
	entry := new(state)
	value, _ := globalRegistry.LoadOrStore(sourceKey, entry)
	metadata := value.(*state)

	return &Stream{
		source:    source,
		sourceKey: sourceKey,
		metadata:  metadata,
	}
}

// ensureParsed ensures the source has been read and parsed.
// This extracts and caches individual namespaces on first access.
func (p *Stream) ensureParsed() error {
	p.metadata.once.Do(func() {
		// Read source if from reader
		if p.source == "" && p.reader != nil {
			// Wrap reader with async read-ahead for concurrent I/O.
			// This allows data to be pre-fetched while we process previous chunks.
			ra := readahead.NewReader(p.reader)
			defer ra.Close()

			data, err := io.ReadAll(ra)
			if err != nil {
				p.metadata.err = ErrReadInput.Wrap(err).
					With(slog.String("source", "reader"))

				return
			}

			p.source = string(data)

			// Generate source key - using xxhash3 for performance
			hash := xxh3.Hash(data)
			p.sourceKey = strconv.FormatUint(hash, 36)
		}

		// Parse source to extract definitions
		ast, err := ParseString(p.source)
		if err != nil {
			p.metadata.err = WrapError(err).With(
				slog.Int("source_length", len(p.source)),
			)

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
func (p *Stream) GetDefinition(name string) (*Definition, error) {
	err := p.ensureParsed()
	if err != nil {
		return nil, err
	}

	cacheKey := p.sourceKey + ":" + name
	if value, ok := globalCache.Load(cacheKey); ok {
		return value.(*Definition), nil
	}

	return nil, ErrDefinitionNotFound.
		With(slog.String("name", name))
}

// Definitions returns an iterator over all definitions in the source.
// If parsing fails, the iterator yields no values.
func (p *Stream) Definitions() iter.Seq[*Definition] {
	return func(yield func(*Definition) bool) {
		err := p.ensureParsed()
		if err != nil {
			return
		}

		for _, id := range p.metadata.identifiers {
			cacheKey := p.sourceKey + ":" + id
			if value, ok := globalCache.Load(cacheKey); ok {
				if !yield(value.(*Definition)) {
					return
				}
			}
		}
	}
}

// AST returns the complete parsed AST.
// This reconstructs the AST from cached definitions.
// Use sparingly - prefer GetDefinition or Definitions for efficiency.
func (p *Stream) AST() (*AST, error) {
	err := p.ensureParsed()
	if err != nil {
		return nil, err
	}

	ast := &AST{
		Definitions: make([]*Definition, len(p.metadata.identifiers)),
	}

	for i, id := range p.metadata.identifiers {
		cacheKey := p.sourceKey + ":" + id
		if value, ok := globalCache.Load(cacheKey); ok {
			ast.Definitions[i] = value.(*Definition)
		}
	}

	return ast, nil
}

// Functional-style interfaces for direct use without creating a Parser
// instance.

// GetDefinitionFrom retrieves a definition by identifier from an io.Reader.
// This is a convenience function that creates a parser and retrieves the
// definition.
func GetDefinitionFrom(r io.Reader, name string) (*Definition, error) {
	return NewStream(r).GetDefinition(name)
}

// DefinitionsFrom returns an iterator over all definitions from an io.Reader.
// This is a convenience function that creates a parser and returns the
// iterator.
func DefinitionsFrom(r io.Reader) iter.Seq[*Definition] {
	return NewStream(r).Definitions()
}

// ClearCache removes all cached definitions and source metadata.
// This is primarily useful for testing or when memory needs to be reclaimed.
func ClearCache() {
	globalCache = sync.Map{}
	globalRegistry = sync.Map{}
}
