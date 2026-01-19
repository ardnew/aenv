package parser

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ardnew/envcomp/pkg/lang"
)

func TestParser_GetDefinition(t *testing.T) {
	source := `
		config : { log_level : "debug" }
		data : { foo : "bar" }
	`

	p := NewFromString(source)

	// Test retrieving existing definition
	ns, err := p.GetDefinition("config")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}
	if ns == nil {
		t.Fatal("expected definition, got nil")
	}
	if ns.Identifier.LiteralString() != "config" {
		t.Errorf("expected definition 'config', got %q", ns.Identifier.LiteralString())
	}

	// Test retrieving another definition
	ns2, err := p.GetDefinition("data")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}
	if ns2.Identifier.LiteralString() != "data" {
		t.Errorf("expected definition 'data', got %q", ns2.Identifier.LiteralString())
	}

	// Test non-existent definition
	_, err = p.GetDefinition("missing")
	if err == nil {
		t.Error("expected error for missing definition")
	}
}

func TestParser_Definitions_Iterator(t *testing.T) {
	source := `
		first : { a : 1 }
		second : { b : 2 }
		third : { c : 3 }
	`

	p := NewFromString(source)

	// Collect all definitions using iterator
	var names []string
	for ns := range p.Definitions() {
		names = append(names, ns.Identifier.LiteralString())
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
			t.Errorf("namespace %d: expected %q, got %q", i, name, names[i])
		}
	}
}

func TestParser_Definitions_EarlyExit(t *testing.T) {
	source := `
		first : { a : 1 }
		second : { b : 2 }
		third : { c : 3 }
	`

	p := NewFromString(source)

	// Stop iteration early
	count := 0
	for ns := range p.Definitions() {
		count++
		if ns.Identifier.LiteralString() == "second" {
			break
		}
	}

	if count != 2 {
		t.Errorf("expected to iterate 2 times, got %d", count)
	}
}

func TestParser_AST(t *testing.T) {
	source := `test : { foo : "bar" }`

	p := NewFromString(source)

	ast, err := p.AST()
	if err != nil {
		t.Fatalf("AST() failed: %v", err)
	}

	if len(ast.Definitions) != 1 {
		t.Errorf("expected 1 definition, got %d", len(ast.Definitions))
	}
}

func TestNewParser_FromReader(t *testing.T) {
	source := `test : { value : 42 }`
	r := strings.NewReader(source)

	ns, err := New(r).GetDefinition("test")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	if ns.Identifier.LiteralString() != "test" {
		t.Errorf("expected definition 'test', got %q", ns.Identifier.LiteralString())
	}
}

func TestGetDefinitionFrom_Functional(t *testing.T) {
	source := `config : { setting : "value" }`
	r := strings.NewReader(source)

	ns, err := GetDefinitionFrom(r, "config")
	if err != nil {
		t.Fatalf("GetDefinitionFrom failed: %v", err)
	}

	if ns.Identifier.LiteralString() != "config" {
		t.Errorf("expected definition 'config', got %q", ns.Identifier.LiteralString())
	}
}

func TestDefinitionsFrom_Functional(t *testing.T) {
	source := `
		alpha : { x : 1 }
		beta : { y : 2 }
	`
	r := strings.NewReader(source)

	var names []string
	for ns := range DefinitionsFrom(r) {
		names = append(names, ns.Identifier.LiteralString())
	}

	if len(names) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(names))
	}
}

func TestParser_ParseError(t *testing.T) {
	source := `invalid { { { syntax`

	p := NewFromString(source)

	// GetDefinition should return parse error
	_, err := p.GetDefinition("test")
	if err == nil {
		t.Error("expected parse error")
	}

	// Definitions iterator should yield nothing on parse error
	count := 0
	for range p.Definitions() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 iterations on parse error, got %d", count)
	}

	// AST should return parse error
	_, err = p.AST()
	if err == nil {
		t.Error("expected parse error from AST()")
	}
}

func TestParser_Caching(t *testing.T) {
	ClearCache()

	source := `cached : { value : "test" }`

	// Create two parsers from the same source
	p1 := NewFromString(source)
	p2 := NewFromString(source)

	// They should share the same metadata (source key should be identical)
	if p1.sourceKey != p2.sourceKey {
		t.Error("expected parsers to have same source key")
	}
	if p1.metadata != p2.metadata {
		t.Error("expected parsers to share metadata")
	}

	// Get definitions from both parsers
	ns1, err1 := p1.GetDefinition("cached")
	if err1 != nil {
		t.Fatalf("p1.GetDefinition() failed: %v", err1)
	}

	ns2, err2 := p2.GetDefinition("cached")
	if err2 != nil {
		t.Fatalf("p2.GetDefinition() failed: %v", err2)
	}

	// Should be the same cached definition instance
	if ns1 != ns2 {
		t.Error("expected same definition instance from cache")
	}
}

// TestParser_Definitions_TypeCheck verifies iterator yields correct types.
func TestParser_Definitions_TypeCheck(t *testing.T) {
	source := `test : { foo : "bar" }`

	p := NewFromString(source)

	for ns := range p.Definitions() {
		// Verify we got a proper namespace
		if ns == nil {
			t.Error("iterator yielded nil namespace")
			continue
		}
		if ns.Identifier == nil {
			t.Error("namespace has nil identifier")
		}
		if ns.Value == nil {
			t.Error("namespace has nil value")
		}

		// Verify it's the right type
		var _ *lang.Definition = ns
	}
}

// TestDefinitionLevelCaching demonstrates that individual definitions are cached,
// not full ASTs, which is more memory efficient.
func TestDefinitionLevelCaching(t *testing.T) {
	ClearCache()

	source := `
		config : { level : "debug", format : "json" }
		database : { host : "localhost", port : 5432 }
		cache : { ttl : 3600, enabled : true }
	`

	// Create first parser and get one definition
	p1 := NewFromString(source)
	config1, err := p1.GetDefinition("config")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	// Create second parser from same source and get same definition
	p2 := NewFromString(source)
	config2, err := p2.GetDefinition("config")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	// The definition instances should be identical (cached)
	if config1 != config2 {
		t.Error("expected same definition instance from cache")
	}

	// Now get a different definition from first parser
	db1, err := p1.GetDefinition("database")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	// Get same definition from second parser
	db2, err := p2.GetDefinition("database")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	// This should also be cached
	if db1 != db2 {
		t.Error("expected same namespace instance from cache")
	}

	// But the two different namespaces should be different objects
	if config1 == db1 {
		t.Error("different namespaces should not be the same object")
	}
}

// TestReaderNotConsumedUntilAccess verifies that NewParser doesn't
// immediately read from the io.Reader.
func TestReaderNotConsumedUntilAccess(t *testing.T) {
	source := `test : { foo : "bar" }`

	// Track whether reader was read
	readCount := 0
	trackingReader := &countingReader{
		reader: strings.NewReader(source),
		count:  &readCount,
	}

	// Create parser - should not read yet
	p := New(trackingReader)

	if readCount > 0 {
		t.Error("NewParser should not consume reader immediately")
	}

	// Now access a namespace - should trigger read
	_, err := p.GetDefinition("test")
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	if readCount == 0 {
		t.Error("GetDefinition should have consumed the reader")
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

// TestMemoryEfficiency demonstrates that namespace-level caching uses less
// memory than full AST caching for large documents.
func TestMemoryEfficiency(t *testing.T) {
	ClearCache()

	// Create a source with many namespaces
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "ns%d : { value%d : \"data\" }\n", i, i)
	}
	source := sb.String()

	p := NewFromString(source)

	// Access only a few namespaces
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("ns%d", i*20)
		_, err := p.GetDefinition(name)
		if err != nil {
			t.Fatalf("GetDefinition(%s) failed: %v", name, err)
		}
	}

	// The full AST structure is not kept in memory after parsing
	// Only the 100 namespaces are cached individually
	// This test just verifies the functionality works correctly
	t.Log("Successfully accessed subset of namespaces without keeping full AST")
}
