package parser

import "testing"

// BenchmarkNamespaceRetrieval measures direct namespace lookup performance.
func BenchmarkNamespaceRetrieval(b *testing.B) {
	source := `
		config : { log_level : "debug", log_format : "json" }
		database : { host : "localhost", port : 5432 }
		cache : { enabled : true, ttl : 3600 }
	`

	p := NewFromString(source)
	// Prime the cache
	_, _ = p.GetDefinition("database")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.GetDefinition("database")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNamespaceCachingVsASTReconstruction compares namespace caching
// to full AST reconstruction.
func BenchmarkNamespaceCachingVsASTReconstruction(b *testing.B) {
	source := `
		ns1 : { a : 1 }
		ns2 : { b : 2 }
		ns3 : { c : 3 }
		ns4 : { d : 4 }
		ns5 : { e : 5 }
	`

	p := NewFromString(source)

	b.Run("GetDefinition", func(b *testing.B) {
		// Prime cache
		_, _ = p.GetDefinition("ns3")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := p.GetDefinition("ns3")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("AST", func(b *testing.B) {
		// Prime cache
		_, _ = p.AST()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ast, err := p.AST()
			if err != nil {
				b.Fatal(err)
			}
			// Access specific namespace from AST
			var found bool
			for _, ns := range ast.Definitions {
				if ns.Identifier.LiteralString() == "ns3" {
					found = true
					break
				}
			}
			if !found {
				b.Fatal("namespace not found")
			}
		}
	})
}

// BenchmarkParser_GetDefinition measures definition lookup performance.
func BenchmarkParser_GetDefinition(b *testing.B) {
	source := `
		config : { log_level : "debug", log_format : "json" }
		data : { foo : "bar", baz : 42 }
		other : { x : 1, y : 2, z : 3 }
	`

	p := NewFromString(source)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.GetDefinition("data")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParser_Definitions_Iterator measures iteration performance.
func BenchmarkParser_Definitions_Iterator(b *testing.B) {
	source := `
		first : { a : 1 }
		second : { b : 2 }
		third : { c : 3 }
		fourth : { d : 4 }
		fifth : { e : 5 }
	`

	p := NewFromString(source)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range p.Definitions() {
			// Iterate through all namespaces
		}
	}
}
