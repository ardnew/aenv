package cli

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/kong"

	"github.com/ardnew/envcomp/cmd/envcomp/parser"
	"github.com/ardnew/envcomp/pkg/lang"
)

func TestGetOrParseConfig_ParsesOnce(t *testing.T) {
	// Reset cache before test
	parser.ClearCache()

	config := `test : { foo : "bar" }`

	// Parse multiple times with same content
	var wg sync.WaitGroup
	results := make([]*lang.AST, 10)
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := strings.NewReader(config)
			p := parser.New(r)
			ast, err := p.AST()
			results[idx] = ast
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all results are successful
	for i, err := range errors {
		if err != nil {
			t.Errorf("parse %d failed: %v", i, err)
		}
	}

	// With namespace-level caching, AST reconstruction creates new objects
	// But the underlying definitions should be the same. Verify via definition retrieval.
	first := results[0]
	for i := 1; i < len(results); i++ {
		if len(results[i].Definitions) != len(first.Definitions) {
			t.Errorf("result %d has different definition count", i)
		}
	}

	// Verify namespace caching by checking same source produces same definition instances
	p1 := parser.NewFromString(config)
	p2 := parser.NewFromString(config)
	ns1, _ := p1.GetDefinition("test")
	ns2, _ := p2.GetDefinition("test")
	if ns1 != ns2 {
		t.Error("expected same definition instance from cache")
	}

	// Verify cache has entries for this source
	// With namespace-level caching, we don't check total cache size
	// Just verify the definition can be retrieved
	p := parser.NewFromString(config)
	_, err := p.GetDefinition("test")
	if err != nil {
		t.Errorf("failed to retrieve cached definition: %v", err)
	}
}

func TestGetOrParseConfig_DifferentContent(t *testing.T) {
	// Reset cache before test
	parser.ClearCache()

	config1 := `test : { foo : "bar" }`
	config2 := `other : { baz : 42 }`

	// Parse two different configs
	p1 := parser.New(strings.NewReader(config1))

	ast1, err1 := p1.AST()
	if err1 != nil {
		t.Fatalf("parse config1 failed: %v", err1)
	}

	p2 := parser.New(strings.NewReader(config2))

	ast2, err2 := p2.AST()
	if err2 != nil {
		t.Fatalf("parse config2 failed: %v", err2)
	}

	// Verify they are different ASTs
	if ast1 == ast2 {
		t.Error("different configs should produce different AST pointers")
	}

	// Verify both definitions are accessible
	// With namespace-level caching, verify retrieval works
	p1 = parser.NewFromString(config1)
	if _, e := p1.GetDefinition("test"); e != nil {
		t.Errorf("failed to retrieve test definition: %v", e)
	}

	p2 = parser.NewFromString(config2)
	if _, e := p2.GetDefinition("other"); e != nil {
		t.Errorf("failed to retrieve other definition: %v", e)
	}
}

func TestGetOrParseConfig_ErrorHandling(t *testing.T) {
	// Reset cache before test
	parser.ClearCache()

	invalidConfig := `invalid syntax { { {`

	p1 := parser.New(strings.NewReader(invalidConfig))

	// Parse invalid config
	_, err := p1.AST()
	if err == nil {
		t.Error("expected parse error for invalid config")
	}

	p2 := parser.New(strings.NewReader(invalidConfig))

	// Parse again - should return same error from cache,
	// even though it's a different parser instance.
	_, err2 := p2.AST()
	if err2 == nil {
		t.Error("expected cached parse error")
	}

	if err.Error() != err2.Error() {
		t.Errorf("error messages differ: %v vs %v", err, err2)
	}
}

func TestLoadNamespace_ReturnsCorrectConfig(t *testing.T) {
	// Reset cache before test
	parser.ClearCache()

	config := `
config : {
	log_level : "debug",
	log_format : "text"
}
other : {
	foo : "bar"
}`

	// Load config namespace
	loader := loadNamespace("config")
	resolver, err := loader(strings.NewReader(config))
	if err != nil {
		t.Fatalf("loadNamespace failed: %v", err)
	}

	// Verify values by creating mock flags and using Resolve
	mockFlag := &kong.Flag{Value: &kong.Value{Name: "log_level"}}
	val, err := resolver.Resolve(nil, nil, mockFlag)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val != "debug" {
		t.Errorf("expected log_level=debug, got %v", val)
	}

	mockFlag2 := &kong.Flag{Value: &kong.Value{Name: "log_format"}}
	val2, err := resolver.Resolve(nil, nil, mockFlag2)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val2 != "text" {
		t.Errorf("expected log_format=text, got %v", val2)
	}

	// Verify 'other' namespace values are not included
	mockFlag3 := &kong.Flag{Value: &kong.Value{Name: "foo"}}
	val3, err := resolver.Resolve(nil, nil, mockFlag3)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val3 != nil {
		t.Error("config should not contain 'foo' from 'other' namespace")
	}
}

func TestLoadNamespace_MissingNamespace(t *testing.T) {
	// Reset cache before test
	parser.ClearCache()

	config := `existing : { foo : "bar" }`

	// Load non-existent namespace
	loader := loadNamespace("missing")
	resolver, err := loader(strings.NewReader(config))
	if err != nil {
		t.Fatalf("loadNamespace failed: %v", err)
	}

	// Verify empty config by trying to resolve a flag
	mockFlag := &kong.Flag{Value: &kong.Value{Name: "foo"}}
	val, err := resolver.Resolve(nil, nil, mockFlag)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val != nil {
		t.Error("expected nil value for missing namespace")
	}
}

func TestLoadNamespace_UnderscoreHyphenMapping(t *testing.T) {
	// Reset cache before test
	parser.ClearCache()

	config := `config : { log_level : "debug" }`

	loader := loadNamespace("config")
	resolver, err := loader(strings.NewReader(config))
	if err != nil {
		t.Fatalf("loadNamespace failed: %v", err)
	}

	// Test underscore version (as stored in config)
	mockFlag := &kong.Flag{Value: &kong.Value{Name: "log_level"}}
	val, err := resolver.Resolve(nil, nil, mockFlag)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val != "debug" {
		t.Errorf("expected log_level=debug, got %v", val)
	}

	// Test hyphen version (should also work via underscore mapping)
	mockFlag2 := &kong.Flag{Value: &kong.Value{Name: "log-level"}}
	val2, err := resolver.Resolve(nil, nil, mockFlag2)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val2 != "debug" {
		t.Errorf("expected log-level=debug, got %v", val2)
	}
}

// BenchmarkGetOrParseConfig_Cached measures performance of cached access.
func BenchmarkGetOrParseConfig_Cached(b *testing.B) {
	// Reset cache and pre-populate
	parser.ClearCache()

	config := `test : { foo : "bar", baz : 42, nested : { a : 1, b : 2 } }`
	p := parser.NewFromString(config)
	_, _ = p.AST()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := parser.NewFromString(config)
		_, err := p.AST()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetOrParseConfig_Uncached measures performance of first parse.
func BenchmarkGetOrParseConfig_Uncached(b *testing.B) {
	config := `test : { foo : "bar", baz : 42, nested : { a : 1, b : 2 } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		parser.ClearCache()
		b.StartTimer()

		p := parser.NewFromString(config)
		_, err := p.AST()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestGetOrParseConfig_ReadError verifies error handling for read failures.
func TestGetOrParseConfig_ReadError(t *testing.T) {
	// Create a reader that always fails
	errReader := &errorReader{err: bytes.ErrTooLarge}

	// New should succeed (doesn't read yet)
	p := parser.New(errReader)


	// But accessing namespace should fail when it tries to read
	_, err := p.GetDefinition("test")
	if err == nil {
		t.Error("expected error from failing reader")
	}
	if !strings.Contains(err.Error(), "failed to read input") {
		t.Errorf("expected 'failed to read input' error, got: %v", err)
	}
}

// errorReader is a reader that always returns an error.
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
