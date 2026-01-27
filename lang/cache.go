package lang

import (
	"io"
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

// ParseReader parses input from an io.Reader and returns the AST.
// The reader content is cached after first parse for efficiency.
func ParseReader(r io.Reader) (*AST, error) {
	// Wrap reader with async read-ahead for concurrent I/O.
	// This allows data to be pre-fetched while we process previous chunks.
	ra := readahead.NewReader(r)
	defer ra.Close()

	data, err := io.ReadAll(ra)
	if err != nil {
		return nil, ErrReadInput.Wrap(err).
			With(slog.String("source", "reader"))
	}

	return parseStringCached(string(data))
}

// parseStringCached parses a string with caching.
func parseStringCached(source string) (*AST, error) {
	// Generate source key (hash) for caching - using xxhash3 for performance
	hash := xxh3.Hash([]byte(source))
	sourceKey := strconv.FormatUint(hash, 36)

	// Get or create metadata entry
	entry := new(state)
	value, _ := globalRegistry.LoadOrStore(sourceKey, entry)

	metadata, ok := value.(*state)
	if !ok {
		return nil, ErrInvalidToken.
			With(slog.String("issue", "invalid metadata type in cache"))
	}

	// Ensure the source has been parsed
	metadata.once.Do(func() {
		// Parse source to extract definitions (bypassing cache)
		ast, parseErr := ParseStringWithOptions(source, DefaultParseOptions())
		if parseErr != nil {
			metadata.err = WrapError(parseErr).With(
				slog.Int("source_length", len(source)),
			)

			return
		}

		// Cache each definition individually and track identifiers
		metadata.identifiers = make([]string, len(ast.Definitions))
		for i, def := range ast.Definitions {
			id := def.Identifier.LiteralString()
			metadata.identifiers[i] = id
			cacheKey := sourceKey + ":" + id
			globalCache.Store(cacheKey, def)
		}
	})

	if metadata.err != nil {
		return nil, metadata.err
	}

	// Reconstruct AST from cached definitions
	ast := &AST{
		Definitions: make([]*Definition, len(metadata.identifiers)),
	}

	for i, id := range metadata.identifiers {
		cacheKey := sourceKey + ":" + id
		if cachedValue, ok := globalCache.Load(cacheKey); ok {
			if def, ok := cachedValue.(*Definition); ok {
				ast.Definitions[i] = def
			}
		}
	}

	return ast, nil
}

// ClearCache removes all cached definitions and source metadata.
// This is primarily useful for testing or when memory needs to be reclaimed.
func ClearCache() {
	globalCache = sync.Map{}
	globalRegistry = sync.Map{}
}
