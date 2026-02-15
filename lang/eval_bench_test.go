package lang

import (
	"context"
	"fmt"
	"testing"
)

// BenchmarkEvaluateExpr benchmarks expression evaluation with pooling and caching.
func BenchmarkEvaluateExpr(b *testing.B) {
	tests := []struct {
		name   string
		config string
		expr   string
	}{
		{
			name:   "simple_arithmetic",
			config: `x : 10; y : 20`,
			expr:   "x + y",
		},
		{
			name:   "string_concatenation",
			config: `greeting : "Hello"; name : "World"`,
			expr:   `greeting + ", " + name + "!"`,
		},
		{
			name:   "builtin_function",
			config: ``,
			expr:   `platform`,
		},
		{
			name:   "namespace_reference",
			config: `a : 1; b : 2; c : 3; sum : a + b + c`,
			expr:   `sum * 2`,
		},
		{
			name:   "complex_expression",
			config: `base : 100; multiplier : 1.5`,
			expr:   `base * multiplier + 50`,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ast, err := ParseString(context.Background(), tt.config)
			if err != nil {
				b.Fatalf("parse error: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := ast.EvaluateExpr(context.Background(), tt.expr)
				if err != nil {
					b.Fatalf("eval error: %v", err)
				}
			}
		})
	}
}

// BenchmarkEvaluateExpr_CacheEffect demonstrates cache effectiveness
// by comparing first evaluation vs subsequent evaluations.
func BenchmarkEvaluateExpr_CacheEffect(b *testing.B) {
	ast, err := ParseString(context.Background(), `x : 10; y : 20; z : 30`)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	expressions := []string{
		"x + y",
		"y * z",
		"x + y + z",
		"(x + y) * z",
		`"result: " + string(x + y + z)`,
	}

	// Clear cache before benchmark
	ClearProgramCache()

	b.Run("first_eval", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Use different expression each time (no cache hits)
			expr := fmt.Sprintf("x + y + %d", i)
			_, err := ast.EvaluateExpr(context.Background(), expr)
			if err != nil {
				b.Fatalf("eval error: %v", err)
			}
		}
	})

	b.Run("cached_eval", func(b *testing.B) {
		// Pre-warm cache
		for _, expr := range expressions {
			_, _ = ast.EvaluateExpr(context.Background(), expr)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Rotate through cached expressions
			expr := expressions[i%len(expressions)]
			_, err := ast.EvaluateExpr(context.Background(), expr)
			if err != nil {
				b.Fatalf("eval error: %v", err)
			}
		}
	})
}

// BenchmarkEvaluateNamespace benchmarks namespace evaluation.
func BenchmarkEvaluateNamespace(b *testing.B) {
	tests := []struct {
		name   string
		config string
		ns     string
		args   []string
	}{
		{
			name:   "simple_value",
			config: `result : 42`,
			ns:     "result",
			args:   nil,
		},
		{
			name:   "with_dependency",
			config: `a : 10; b : 20; sum : a + b`,
			ns:     "sum",
			args:   nil,
		},
		{
			name:   "parameterized",
			config: `double x : x * 2`,
			ns:     "double",
			args:   []string{"21"},
		},
		{
			name:   "multiple_params",
			config: `add x y : x + y`,
			ns:     "add",
			args:   []string{"15", "27"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ast, err := ParseString(context.Background(), tt.config)
			if err != nil {
				b.Fatalf("parse error: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := ast.EvaluateNamespace(context.Background(), tt.ns, tt.args)
				if err != nil {
					b.Fatalf("eval error: %v", err)
				}
			}
		})
	}
}

// BenchmarkRuntimeEnvPooling specifically measures the pooling effectiveness.
func BenchmarkRuntimeEnvPooling(b *testing.B) {
	config := `a : 1; b : 2; c : 3; sum : a + b + c`
	ast, err := ParseString(context.Background(), config)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	// Pre-warm cache to isolate pooling effects
	_, _ = ast.EvaluateNamespace(context.Background(), "sum", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ast.EvaluateNamespace(context.Background(), "sum", nil)
		if err != nil {
			b.Fatalf("eval error: %v", err)
		}
	}
}

// BenchmarkCompilationCaching measures compilation cache effectiveness.
func BenchmarkCompilationCaching(b *testing.B) {
	ast, err := ParseString(context.Background(), `x : 100; y : 200`)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	expr := "x * 2 + y * 3"

	b.Run("without_cache", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Clear cache every iteration to simulate no caching
			ClearProgramCache()
			_, err := ast.EvaluateExpr(context.Background(), expr)
			if err != nil {
				b.Fatalf("eval error: %v", err)
			}
		}
	})

	b.Run("with_cache", func(b *testing.B) {
		// Pre-warm cache
		_, _ = ast.EvaluateExpr(context.Background(), expr)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := ast.EvaluateExpr(context.Background(), expr)
			if err != nil {
				b.Fatalf("eval error: %v", err)
			}
		}
	})
}

// BenchmarkSequentialEvaluations simulates REPL usage pattern.
func BenchmarkSequentialEvaluations(b *testing.B) {
	config := `
		greeting : "Hello";
		name : "World";
		count : 42;
		pi : 3.14159;
		fullGreeting : greeting + ", " + name + "!"
	`
	ast, err := ParseString(context.Background(), config)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	// Common expressions evaluated in REPL
	expressions := []string{
		"greeting",
		"name",
		"count",
		"pi",
		"fullGreeting",
		"count * 2",
		`greeting + " there"`,
		"pi * count",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		expr := expressions[i%len(expressions)]
		_, err := ast.EvaluateExpr(context.Background(), expr)
		if err != nil {
			b.Fatalf("eval error: %v", err)
		}
	}
}
