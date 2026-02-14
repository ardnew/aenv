package repl

import (
	"context"
	"reflect"
	"testing"

	"github.com/ardnew/aenv/lang"
)

// BenchmarkGetExprLangBuiltinSignature benchmarks the cached version of
// expr-lang builtin signature lookups.
func BenchmarkGetExprLangBuiltinSignature(b *testing.B) {
	functions := []string{"len", "join", "filter", "map", "upper", "lower"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		funcName := functions[i%len(functions)]
		_, _, _ = getExprLangBuiltinSignature(funcName)
	}
}

// BenchmarkGetExprLangBuiltinSignatureUncached benchmarks the uncached version
// of expr-lang builtin signature lookups.
func BenchmarkGetExprLangBuiltinSignatureUncached(b *testing.B) {
	functions := []string{"len", "join", "filter", "map", "upper", "lower"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		funcName := functions[i%len(functions)]
		_, _, _ = getExprLangBuiltinSignatureUncached(funcName)
	}
}

// BenchmarkGetBuiltinSignature benchmarks the cached version of project
// builtin signature lookups.
func BenchmarkGetBuiltinSignature(b *testing.B) {
	functions := []string{"file.exists", "path.cat", "path.rel", "mung.prefix"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		funcName := functions[i%len(functions)]
		_, _, _ = getBuiltinSignature(funcName)
	}
}

// BenchmarkGetBuiltinSignatureUncached benchmarks the uncached version of
// project builtin signature lookups.
func BenchmarkGetBuiltinSignatureUncached(b *testing.B) {
	functions := []string{"file.exists", "path.cat", "path.rel", "mung.prefix"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		funcName := functions[i%len(functions)]
		_, _, _ = getBuiltinSignatureUncached(funcName)
	}
}

// BenchmarkGetExprLangBuiltinSignature_SingleFunction benchmarks repeated
// lookups of the same function (best case for caching).
func BenchmarkGetExprLangBuiltinSignature_SingleFunction(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = getExprLangBuiltinSignature("len")
	}
}

// BenchmarkGetExprLangBuiltinSignatureUncached_SingleFunction benchmarks
// repeated uncached lookups of the same function.
func BenchmarkGetExprLangBuiltinSignatureUncached_SingleFunction(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = getExprLangBuiltinSignatureUncached("len")
	}
}

// BenchmarkGetBuiltinSignature_SingleFunction benchmarks repeated lookups of
// the same project builtin function (best case for caching).
func BenchmarkGetBuiltinSignature_SingleFunction(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = getBuiltinSignature("file.exists")
	}
}

// BenchmarkGetBuiltinSignatureUncached_SingleFunction benchmarks repeated
// uncached lookups of the same project builtin function.
func BenchmarkGetBuiltinSignatureUncached_SingleFunction(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = getBuiltinSignatureUncached("file.exists")
	}
}

// BenchmarkGetSignature_ExprLangBuiltin benchmarks the full signature lookup
// path for expr-lang builtins (goes through getSignature -> cached lookup).
func BenchmarkGetSignature_ExprLangBuiltin(b *testing.B) {
	ast, err := lang.ParseString(context.Background(), "")
	if err != nil {
		b.Fatalf("Failed to create AST: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = getSignature(ast, "len")
	}
}

// BenchmarkGetSignature_ProjectBuiltin benchmarks the full signature lookup
// path for project builtins (goes through getSignature -> cached lookup).
func BenchmarkGetSignature_ProjectBuiltin(b *testing.B) {
	ast, err := lang.ParseString(context.Background(), "")
	if err != nil {
		b.Fatalf("Failed to create AST: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = getSignature(ast, "file.exists")
	}
}

// BenchmarkFormatSemanticTypeName benchmarks the type name formatting helper.
func BenchmarkFormatSemanticTypeName(b *testing.B) {
	// Use a common reflection type
	var testFunc func([]interface{}, func(interface{}) bool) []interface{}
	funcType := reflect.TypeOf(testFunc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatSemanticTypeName("filter", 0, funcType.In(0))
		_ = formatSemanticTypeName("filter", 1, funcType.In(1))
	}
}
