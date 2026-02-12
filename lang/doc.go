// Package lang provides parsing and representation of a custom configuration
// language.
//
// The [Stream] type allows efficient on-demand parsing and retrieval of
// individual [Definition]s without loading entire ASTs into memory.
// [Definition]s are cached automatically for improved performance.
package lang
