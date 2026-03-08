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

// BenchmarkBuildRuntimeEnv_CloneCost isolates env clone vs direct copy.
func BenchmarkBuildRuntimeEnv_CloneCost(b *testing.B) {
	config := `a : 1; b : 2; c : 3`
	ast, err := ParseString(context.Background(), config)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	// Pre-warm
	_, _ = ast.EvaluateNamespace(context.Background(), "a", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ast.EvaluateNamespace(context.Background(), "a", nil)
		if err != nil {
			b.Fatalf("eval error: %v", err)
		}
	}
}

// BenchmarkIsFunction measures reflection cost for type checking.
func BenchmarkIsFunction(b *testing.B) {
	values := []any{
		"hello",
		int64(42),
		true,
		3.14,
		nil,
		map[string]any{"a": 1},
		[]any{1, 2, 3},
		func() {},
		func(x int) int { return x },
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, v := range values {
			_ = isFunction(v)
		}
	}
}

// BenchmarkEvaluateExpr_LoggingOverhead measures sortedKeys cost.
func BenchmarkEvaluateExpr_LoggingOverhead(b *testing.B) {
	config := `a : 1; b : 2; c : 3; d : 4; e : 5`
	ast, err := ParseString(context.Background(), config)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	// Pre-warm cache
	_, _ = ast.EvaluateExpr(context.Background(), "a + b")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ast.EvaluateExpr(context.Background(), "a + b")
		if err != nil {
			b.Fatalf("eval error: %v", err)
		}
	}
}

func BenchmarkEvaluateNamespace_CompileCost(b *testing.B) {
	tests := []struct {
		name   string
		config string
		ns     string
		args   []string
	}{
		{
			name:   "single_dep",
			config: `a x : x + 1; b x : a(x) * 2`,
			ns:     "b",
			args:   []string{"5"},
		},
		{
			name:   "chain_3",
			config: `a x : x + 1; b x : a(x) * 2; c x : b(x) + 3`,
			ns:     "c",
			args:   []string{"5"},
		},
		{
			name:   "chain_5",
			config: `a x : x + 1; b x : a(x) * 2; c x : b(x) + 3; d x : c(x) - 1; e x : d(x) * 4`,
			ns:     "e",
			args:   []string{"5"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ast, err := ParseString(context.Background(), tt.config)
			if err != nil {
				b.Fatalf("parse error: %v", err)
			}

			_, err = ast.EvaluateNamespace(context.Background(), tt.ns, tt.args)
			if err != nil {
				b.Fatalf("eval error: %v", err)
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

func BenchmarkEvaluateNamespace_DiamondDeps(b *testing.B) {
	tests := []struct {
		name   string
		config string
		ns     string
	}{
		{
			name:   "diamond_4",
			config: `a : 1; b : a + 1; c : a + 2; sum : b + c`,
			ns:     "sum",
		},
		{
			name:   "fan_in_5",
			config: `base : 10; x1 : base * 1; x2 : base * 2; x3 : base * 3; x4 : base * 4; total : x1 + x2 + x3 + x4`,
			ns:     "total",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ast, err := ParseString(context.Background(), tt.config)
			if err != nil {
				b.Fatalf("parse error: %v", err)
			}

			_, err = ast.EvaluateNamespace(context.Background(), tt.ns, nil)
			if err != nil {
				b.Fatalf("warmup error: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := ast.EvaluateNamespace(context.Background(), tt.ns, nil)
				if err != nil {
					b.Fatalf("eval error: %v", err)
				}
			}
		})
	}
}

// BenchmarkMergeEntries_Repeated measures mergeEntries cost across
// repeated evaluations of the same AST.
func BenchmarkMergeEntries_Repeated(b *testing.B) {
	config := `a : 1; b : 2; c : 3; d : 4; e : 5; f : 6; g : 7; h : 8`
	ast, err := ParseString(context.Background(), config)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	// Pre-warm
	_, _ = ast.EvaluateNamespace(context.Background(), "h", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ast.EvaluateNamespace(context.Background(), "h", nil)
		if err != nil {
			b.Fatalf("eval error: %v", err)
		}
	}
}
