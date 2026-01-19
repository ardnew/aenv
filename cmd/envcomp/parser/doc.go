// Package parser provides streaming parsing and caching for envcomp source
// text.
//
// The Parser type allows efficient on-demand parsing and retrieval of
// individual
// definitions without loading entire ASTs into memory. Definitions are cached
// automatically for improved performance on repeated access.
package parser
