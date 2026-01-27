package lang

import (
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
