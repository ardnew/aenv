package lang

import (
	"bytes"
	"context"
	"encoding/gob"
	"io"
	"log/slog"
	"strconv"
	"sync"

	"github.com/klauspost/readahead"
	"github.com/zeebo/xxh3"

	"github.com/ardnew/aenv/lang/lexer"
)

var (
	// globalCache stores namespaces keyed by (source_hash:identifier).
	// This allows efficient lookup without keeping full ASTs in memory.
	globalCache sync.Map

	// globalRegistry tracks source metadata by source hash.
	globalRegistry sync.Map
)

// state tracks parsing state and top-level namespace list for a source.
type state struct {
	once        sync.Once
	identifiers []string // List of namespace identifiers found
	err         error
}

// hashOptions encodes options using gob and hashes with xxh3.
// Returns a hash that uniquely identifies the options configuration.
func hashOptions(opts optionsKey) uint64 {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	// Encode relevant options fields
	_ = enc.Encode(opts.maxDepth)
	_ = enc.Encode(opts.compileExprs)
	_ = enc.Encode(opts.processEnv)

	return xxh3.Hash(buf.Bytes())
}

// ParseReader parses input from an io.Reader and returns the AST.
// The reader content is cached after first parse for efficiency.
func ParseReader(
	ctx context.Context,
	r io.Reader,
	opts ...Option,
) (*AST, error) {
	// Wrap reader with async read-ahead for concurrent I/O.
	// This allows data to be pre-fetched while we process previous chunks.
	ra := readahead.NewReader(r)
	defer ra.Close()

	data, err := io.ReadAll(ra)
	if err != nil {
		return nil, ErrReadInput.Wrap(err).
			With(slog.String("source", "reader"))
	}

	// Build a temporary AST to determine options state
	var tempAST AST

	applyDefaults(&tempAST)
	applyOptions(&tempAST, opts...)

	tempAST.logger.TraceContext(
		ctx,
		"read input",
		slog.Int("source_bytes", len(data)),
		slog.Bool("read_ahead", true),
	)

	// If options differ from defaults (e.g., compileExprs), bypass cache
	if tempAST.opts.compileExprs || tempAST.opts.maxDepth != DefaultMaxDepth {
		tempAST.logger.TraceContext(
			ctx,
			"cache bypass",
			slog.Bool("compile_exprs", tempAST.opts.compileExprs),
			slog.Int("max_depth", tempAST.opts.maxDepth),
		)

		return ParseString(ctx, string(data), opts...)
	}

	return parseStringCached(ctx, string(data), opts...)
}

// parseStringCached parses a string with caching.
func parseStringCached(
	ctx context.Context,
	source string,
	opts ...Option,
) (*AST, error) {
	// Build a temporary AST to get effective options
	var tempAST AST

	applyDefaults(&tempAST)
	applyOptions(&tempAST, opts...)

	// Generate source key (hash) for caching - using xxhash3 for performance
	// Combine source hash with options hash for cache key uniqueness
	sourceHash := xxh3.Hash([]byte(source))
	optsHash := hashOptions(tempAST.opts)
	combinedHash := sourceHash ^ optsHash
	sourceKey := strconv.FormatUint(combinedHash, 36)

	// Get or create metadata entry
	entry := new(state)
	value, cacheHit := globalRegistry.LoadOrStore(sourceKey, entry)

	metadata, ok := value.(*state)
	if !ok {
		return nil, ErrInvalidToken.
			With(slog.String("issue", "invalid metadata type in cache"))
	}

	tempAST.logger.TraceContext(
		ctx,
		"cache lookup",
		slog.String("source_hash", strconv.FormatUint(sourceHash, 16)),
		slog.String("opts_hash", strconv.FormatUint(optsHash, 16)),
		slog.Bool("cache_hit", cacheHit),
	)

	// Ensure the source has been parsed
	metadata.once.Do(func() {
		// Parse source to extract namespaces (bypassing cache)
		ast, parseErr := parse(ctx, lexer.New([]rune(source)), source, opts...)
		if parseErr != nil {
			metadata.err = WrapError(parseErr).With(
				slog.Int("source_length", len(source)),
			)

			return
		}

		// Cache each namespace individually and track identifiers
		metadata.identifiers = make([]string, len(ast.Namespaces))
		for i, ns := range ast.Namespaces {
			id := ns.Identifier.LiteralString()
			metadata.identifiers[i] = id
			cacheKey := sourceKey + ":" + id
			globalCache.Store(cacheKey, ns)
		}
	})

	if metadata.err != nil {
		return nil, metadata.err
	}

	// Reconstruct AST from cached namespaces
	ast := &AST{
		Namespaces: make([]*Namespace, len(metadata.identifiers)),
	}

	applyDefaults(ast)
	applyOptions(ast, opts...)

	for i, id := range metadata.identifiers {
		cacheKey := sourceKey + ":" + id
		if cachedValue, ok := globalCache.Load(cacheKey); ok {
			if ns, ok := cachedValue.(*Namespace); ok {
				ast.Namespaces[i] = ns
			}
		}
	}

	return ast, nil
}

// ClearCache removes all cached namespaces and source metadata.
// This is primarily useful for testing or when memory needs to be reclaimed.
func ClearCache() {
	globalCache = sync.Map{}
	globalRegistry = sync.Map{}
}
