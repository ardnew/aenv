package lang

import (
	"testing"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// BenchmarkExprCompile_MapEnv measures compile cost with map env.
func BenchmarkExprCompile_MapEnv(b *testing.B) {
	env := map[string]any{
		"x": 10,
		"y": 20,
		"z": "hello",
		"w": true,
		"m": map[string]any{"a": 1, "b": 2},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := expr.Compile("x + y", expr.Env(env))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExprCompile_StructEnv measures compile cost with struct env.
func BenchmarkExprCompile_StructEnv(b *testing.B) {
	type Env struct {
		X int            `expr:"x"`
		Y int            `expr:"y"`
		Z string         `expr:"z"`
		W bool           `expr:"w"`
		M map[string]any `expr:"m"`
	}

	env := Env{X: 10, Y: 20, Z: "hello", W: true, M: map[string]any{"a": 1, "b": 2}}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := expr.Compile("x + y", expr.Env(env))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExprRun_PostCompile measures pure VM execution cost.
func BenchmarkExprRun_PostCompile(b *testing.B) {
	env := map[string]any{"x": 10, "y": 20}
	program, _ := expr.Compile("x + y", expr.Env(env))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := vm.Run(program, env)
		if err != nil {
			b.Fatal(err)
		}
	}
}
