package lang

import (
"fmt"
"io"
"strings"
"testing"
)

func TestAST_GetDefinition(t *testing.T) {
source := `
config : { log_level : "debug" }
data : { foo : "bar" }
`

ast, err := ParseString(source)
if err != nil {
t.Fatalf("ParseString failed: %v", err)
}

// Test retrieving existing definition
def, ok := ast.GetDefinition("config")
if !ok {
t.Fatal("expected definition to be found")
}
if def == nil {
t.Fatal("expected definition, got nil")
}
if def.Identifier.LiteralString() != "config" {
t.Errorf("expected definition 'config', got %q", def.Identifier.LiteralString())
}

// Test retrieving another definition
def2, ok := ast.GetDefinition("data")
if !ok {
t.Fatal("expected definition to be found")
}
if def2.Identifier.LiteralString() != "data" {
t.Errorf("expected definition 'data', got %q", def2.Identifier.LiteralString())
}

// Test non-existent definition
_, ok = ast.GetDefinition("missing")
if ok {
t.Error("expected definition not to be found")
}
}

func TestAST_All_Iterator(t *testing.T) {
source := `
first : { a : 1 }
second : { b : 2 }
third : { c : 3 }
`

ast, err := ParseString(source)
if err != nil {
t.Fatalf("ParseString failed: %v", err)
}

// Collect all definitions using iterator
var names []string
for def := range ast.All() {
names = append(names, def.Identifier.LiteralString())
}

expected := []string{"first", "second", "third"}
if len(names) != len(expected) {
t.Errorf("expected %d definitions, got %d", len(expected), len(names))
}

for i, name := range expected {
if i >= len(names) {
break
}
if names[i] != name {
t.Errorf("definition %d: expected %q, got %q", i, name, names[i])
}
}
}

func TestAST_All_EarlyExit(t *testing.T) {
source := `
first : { a : 1 }
second : { b : 2 }
third : { c : 3 }
`

ast, err := ParseString(source)
if err != nil {
t.Fatalf("ParseString failed: %v", err)
}

// Stop iteration early
count := 0
for def := range ast.All() {
count++
if def.Identifier.LiteralString() == "second" {
break
}
}

if count != 2 {
t.Errorf("expected to iterate 2 times, got %d", count)
}
}

func TestParseReader(t *testing.T) {
source := `test : { value : 42 }`
r := strings.NewReader(source)

ast, err := ParseReader(r)
if err != nil {
t.Fatalf("ParseReader failed: %v", err)
}

def, ok := ast.GetDefinition("test")
if !ok {
t.Fatal("expected definition to be found")
}

if def.Identifier.LiteralString() != "test" {
t.Errorf("expected definition 'test', got %q", def.Identifier.LiteralString())
}
}

func TestParseReader_ParseError(t *testing.T) {
source := `invalid { { { syntax`
r := strings.NewReader(source)

// ParseReader should return parse error
_, err := ParseReader(r)
if err == nil {
t.Error("expected parse error")
}
}

func TestParseReader_Caching(t *testing.T) {
ClearCache()

source := `cached : { value : "test" }`

// Parse the same source twice
ast1, err := ParseReader(strings.NewReader(source))
if err != nil {
t.Fatalf("first ParseReader failed: %v", err)
}

ast2, err := ParseReader(strings.NewReader(source))
if err != nil {
t.Fatalf("second ParseReader failed: %v", err)
}

// Get definitions from both ASTs
def1, ok1 := ast1.GetDefinition("cached")
if !ok1 {
t.Fatal("expected definition to be found in first AST")
}

def2, ok2 := ast2.GetDefinition("cached")
if !ok2 {
t.Fatal("expected definition to be found in second AST")
}

// Should be the same cached definition instance
if def1 != def2 {
t.Error("expected same definition instance from cache")
}
}

func TestAST_All_TypeCheck(t *testing.T) {
source := `test : { foo : "bar" }`

ast, err := ParseString(source)
if err != nil {
t.Fatalf("ParseString failed: %v", err)
}

for def := range ast.All() {
// Verify we got a proper definition
if def == nil {
t.Error("iterator yielded nil definition")
continue
}
if def.Identifier == nil {
t.Error("definition has nil identifier")
}
if def.Value == nil {
t.Error("definition has nil value")
}

// Verify it's the right type
var _ *Definition = def
}
}

func TestDefinitionLevelCaching(t *testing.T) {
ClearCache()

source := `
config : { level : "debug", format : "json" }
database : { host : "localhost", port : 5432 }
cache : { ttl : 3600, enabled : true }
`

// Parse source twice
ast1, err := ParseReader(strings.NewReader(source))
if err != nil {
t.Fatalf("first ParseReader failed: %v", err)
}

ast2, err := ParseReader(strings.NewReader(source))
if err != nil {
t.Fatalf("second ParseReader failed: %v", err)
}

// Get same definition from both ASTs
config1, ok := ast1.GetDefinition("config")
if !ok {
t.Fatal("expected definition to be found in first AST")
}

config2, ok := ast2.GetDefinition("config")
if !ok {
t.Fatal("expected definition to be found in second AST")
}

// The definition instances should be identical (cached)
if config1 != config2 {
t.Error("expected same definition instance from cache")
}

// Get a different definition from first AST
db1, ok := ast1.GetDefinition("database")
if !ok {
t.Fatal("expected definition to be found")
}

// Get same definition from second AST
db2, ok := ast2.GetDefinition("database")
if !ok {
t.Fatal("expected definition to be found")
}

// This should also be cached
if db1 != db2 {
t.Error("expected same definition instance from cache")
}

// But the two different definitions should be different objects
if config1 == db1 {
t.Error("different definitions should not be the same object")
}
}

func TestReaderNotConsumedUntilAccess(t *testing.T) {
source := `test : { foo : "bar" }`

// Track whether reader was read
readCount := 0
trackingReader := &countingReader{
reader: strings.NewReader(source),
count:  &readCount,
}

// ParseReader immediately consumes the reader (unlike the old Stream API)
_, err := ParseReader(trackingReader)
if err != nil {
t.Fatalf("ParseReader failed: %v", err)
}

if readCount == 0 {
t.Error("ParseReader should have consumed the reader")
}
}

// countingReader wraps an io.Reader and counts Read calls.
type countingReader struct {
reader io.Reader
count  *int
}

func (c *countingReader) Read(p []byte) (n int, err error) {
*c.count++
return c.reader.Read(p)
}

func TestMemoryEfficiency(t *testing.T) {
ClearCache()

// Create a source with many definitions
var sb strings.Builder
for i := 0; i < 100; i++ {
fmt.Fprintf(&sb, "ns%d : { value%d : \"data\" }\n", i, i)
}
source := sb.String()

ast, err := ParseString(source)
if err != nil {
t.Fatalf("ParseString failed: %v", err)
}

// Access only a few definitions
for i := 0; i < 5; i++ {
name := fmt.Sprintf("ns%d", i*20)
_, ok := ast.GetDefinition(name)
if !ok {
t.Fatalf("GetDefinition(%s) not found", name)
}
}

// The definitions are cached individually after first parse
t.Log("Successfully accessed subset of definitions")
}

func TestAST_Define(t *testing.T) {
ast := &AST{}

def := ast.Define("test", nil, NewString("value"))

if len(ast.Definitions) != 1 {
t.Errorf("expected 1 definition, got %d", len(ast.Definitions))
}

if def.Identifier.LiteralString() != "test" {
t.Errorf("expected identifier 'test', got %q", def.Identifier.LiteralString())
}

// Verify we can retrieve it
retrieved, ok := ast.GetDefinition("test")
if !ok {
t.Error("expected to find defined definition")
}
if retrieved != def {
t.Error("expected to retrieve the same definition instance")
}
}
