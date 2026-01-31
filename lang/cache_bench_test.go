package lang

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// BenchmarkDefinitionRetrieval measures direct definition lookup performance.
func BenchmarkDefinitionRetrieval(b *testing.B) {
	source := `
config : { log_level : "debug", log_format : "json" }
database : { host : "localhost", port : 5432 }
cache : { enabled : true, ttl : 3600 }
`

	ast, err := ParseString(source)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, ok := ast.GetDefinition("database")
		if !ok {
			b.Fatal("definition not found")
		}
	}
}

// BenchmarkAST_GetDefinition measures definition lookup performance.
func BenchmarkAST_GetDefinition(b *testing.B) {
	source := `
config : { log_level : "debug", log_format : "json" }
data : { foo : "bar", baz : 42 }
other : { x : 1, y : 2, z : 3 }
`

	ast, err := ParseString(source)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, ok := ast.GetDefinition("data")
		if !ok {
			b.Fatal("definition not found")
		}
	}
}

// BenchmarkAST_All_Iterator measures iteration performance.
func BenchmarkAST_All_Iterator(b *testing.B) {
	source := `
first : { a : 1 }
second : { b : 2 }
third : { c : 3 }
fourth : { d : 4 }
fifth : { e : 5 }
`

	ast, err := ParseString(source)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range ast.All() {
			// Iterate through all definitions
		}
	}
}

// BenchmarkParseReader measures ParseReader performance across different input sizes.
func BenchmarkParseReader(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"small", 10},
		{"medium", 200},
		{"large", 2000},
	}

	for _, size := range sizes {
		// Generate test source
		var sb strings.Builder
		for i := 0; i < size.count; i++ {
			fmt.Fprintf(&sb, "def%d : { value : %d }\n", i, i)
		}
		source := sb.String()

		b.Run(size.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := ParseReader(strings.NewReader(source))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkParseString_Caching measures the impact of caching on repeated parses.
func BenchmarkParseString_Caching(b *testing.B) {
	source := `
config : { log_level : "debug", log_format : "json" }
database : { host : "localhost", port : 5432 }
cache : { enabled : true, ttl : 3600 }
`

	ClearCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseString(source)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAST_Define measures programmatic AST construction performance.
func BenchmarkAST_Define(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ast := &AST{}
		for j := 0; j < 100; j++ {
			ast.Define(
				fmt.Sprintf("def%d", j),
				nil,
				NewTuple(
					NewDefinition("key", nil, NewString("value")),
					NewDefinition("number", nil, NewNumber("42")),
				),
			)
		}
	}
}

// BenchmarkCompileExprs measures expression compilation performance.
func BenchmarkCompileExprs(b *testing.B) {
	sizes := []struct {
		name      string
		exprCount int
	}{
		{"few_exprs", 10},
		{"many_exprs", 100},
		{"heavy_exprs", 500},
	}

	for _, size := range sizes {
		// Generate source with expressions
		var sb strings.Builder
		sb.WriteString("config : {\n")
		for i := 0; i < size.exprCount; i++ {
			fmt.Fprintf(&sb, "  expr%d : {{ %d + %d }},\n", i, i, i*2)
		}
		sb.WriteString("}\n")
		source := sb.String()

		b.Run(size.name, func(b *testing.B) {
			// Parse without compilation first
			ast, err := ParseString(source)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := ast.CompileExprs()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkEvaluateDefinition measures evaluation performance.
func BenchmarkEvaluateDefinition(b *testing.B) {
	b.Run("simple_value", func(b *testing.B) {
		source := `greeting : "Hello, World!"`
		ast, err := ParseString(source, WithCompileExprs(true))
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ast.EvaluateDefinition("greeting", nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("nested_tuple", func(b *testing.B) {
		source := `
config : {
server : {
host : "localhost",
port : 8080,
options : {
timeout : 30,
retries : 3
}
}
}
`
		ast, err := ParseString(source, WithCompileExprs(true))
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ast.EvaluateDefinition("config", nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with_expressions", func(b *testing.B) {
		source := `
math : {
a : 10,
b : 20,
sum : {{ a + b }},
product : {{ a * b }},
complex : {{ (a + b) * 2 }}
}
`
		ast, err := ParseString(source, WithCompileExprs(true))
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ast.EvaluateDefinition("math", nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with_parameters", func(b *testing.B) {
		// Note: Parameters are typed as any(nil) at compile time, so string
		// operations on parameters have limitations. Use a simple expression.
		source := `greet name : {{ name }}`
		ast, err := ParseString(source, WithCompileExprs(true))
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ast.EvaluateDefinition("greet", []string{"World"})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkFormat measures native format output performance.
func BenchmarkFormat(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"small", 10},
		{"medium", 100},
		{"large", 1000},
	}

	for _, size := range sizes {
		var sb strings.Builder
		for i := 0; i < size.count; i++ {
			fmt.Fprintf(&sb, "def%d : { value : %d }\n", i, i)
		}
		source := sb.String()

		b.Run(size.name, func(b *testing.B) {
			ast, err := ParseString(source)
			if err != nil {
				b.Fatal(err)
			}

			var buf bytes.Buffer
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				err := ast.Format(&buf, 2)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkFormatJSON measures JSON format output performance.
func BenchmarkFormatJSON(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"small", 10},
		{"medium", 100},
		{"large", 1000},
	}

	for _, size := range sizes {
		var sb strings.Builder
		for i := 0; i < size.count; i++ {
			fmt.Fprintf(&sb, "def%d : { value : %d }\n", i, i)
		}
		source := sb.String()

		b.Run(size.name, func(b *testing.B) {
			ast, err := ParseString(source)
			if err != nil {
				b.Fatal(err)
			}

			var buf bytes.Buffer
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				err := ast.FormatJSON(&buf, 2)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkFormatYAML measures YAML format output performance.
func BenchmarkFormatYAML(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"small", 10},
		{"medium", 100},
		{"large", 1000},
	}

	for _, size := range sizes {
		var sb strings.Builder
		for i := 0; i < size.count; i++ {
			fmt.Fprintf(&sb, "def%d : { value : %d }\n", i, i)
		}
		source := sb.String()

		b.Run(size.name, func(b *testing.B) {
			ast, err := ParseString(source)
			if err != nil {
				b.Fatal(err)
			}

			var buf bytes.Buffer
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				err := ast.FormatYAML(context.Background(), &buf, 2)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkToMap measures AST to map conversion performance.
func BenchmarkToMap(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"small", 10},
		{"medium", 100},
		{"large", 1000},
	}

	for _, size := range sizes {
		var sb strings.Builder
		for i := 0; i < size.count; i++ {
			fmt.Fprintf(&sb, "def%d : { value : %d }\n", i, i)
		}
		source := sb.String()

		b.Run(size.name, func(b *testing.B) {
			ast, err := ParseString(source)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ast.ToMap()
			}
		})
	}
}
