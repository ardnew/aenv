package cli

import (
	"bytes"
	"maps"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/kong"

	"github.com/ardnew/aenv/lang"
)

func TestGetOrParseConfig_ParsesOnce(t *testing.T) {
	// Reset cache before test
	lang.ClearCache()

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
			ast, err := lang.ParseReader(r)
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

	// Verify definition caching by checking same source produces same definition instances
	ast1, err := lang.ParseString(config)
	if err != nil {
		t.Fatalf("first ParseString failed: %v", err)
	}
	ast2, err := lang.ParseString(config)
	if err != nil {
		t.Fatalf("second ParseString failed: %v", err)
	}
	def1, ok1 := ast1.GetDefinition("test")
	def2, ok2 := ast2.GetDefinition("test")
	if !ok1 || !ok2 {
		t.Fatal("expected definition to be found")
	}
	if def1 != def2 {
		t.Error("expected same definition instance from cache")
	}

	// Verify cache has entries for this source
	// With definition-level caching, we don't check total cache size
	// Just verify the definition can be retrieved
	ast, err := lang.ParseString(config)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	_, ok := ast.GetDefinition("test")
	if !ok {
		t.Error("failed to retrieve cached definition")
	}
}

func TestGetOrParseConfig_DifferentContent(t *testing.T) {
	// Reset cache before test
	lang.ClearCache()

	config1 := `test : { foo : "bar" }`
	config2 := `other : { baz : 42 }`

	// Parse two different configs
	ast1, err1 := lang.ParseReader(strings.NewReader(config1))
	if err1 != nil {
		t.Fatalf("parse config1 failed: %v", err1)
	}

	ast2, err2 := lang.ParseReader(strings.NewReader(config2))
	if err2 != nil {
		t.Fatalf("parse config2 failed: %v", err2)
	}

	// Verify they are different ASTs
	if ast1 == ast2 {
		t.Error("different configs should produce different AST pointers")
	}

	// Verify both definitions are accessible
	// With definition-level caching, verify retrieval works
	ast1Reparse, err := lang.ParseString(config1)
	if err != nil {
		t.Fatalf("reparse config1 failed: %v", err)
	}
	if _, ok := ast1Reparse.GetDefinition("test"); !ok {
		t.Error("failed to retrieve test definition")
	}

	ast2Reparse, err := lang.ParseString(config2)
	if err != nil {
		t.Fatalf("reparse config2 failed: %v", err)
	}
	if _, ok := ast2Reparse.GetDefinition("other"); !ok {
		t.Error("failed to retrieve other definition")
	}
}

func TestGetOrParseConfig_ErrorHandling(t *testing.T) {
	// Reset cache before test
	lang.ClearCache()

	invalidConfig := `invalid syntax { { {`

	// Parse invalid config
	_, err := lang.ParseReader(strings.NewReader(invalidConfig))
	if err == nil {
		t.Error("expected parse error for invalid config")
	}

	// Parse again - should return same error from cache
	_, err2 := lang.ParseReader(strings.NewReader(invalidConfig))
	if err2 == nil {
		t.Error("expected cached parse error")
	}

	// Compare token sets from error messages (order-independent)
	extractTokens := func(errMsg string) map[string]bool {
		start := strings.Index(errMsg, "[")
		end := strings.Index(errMsg, "]")
		if start == -1 || end == -1 {
			return nil
		}
		tokenStr := errMsg[start+1 : end]
		tokens := strings.Split(tokenStr, ",")
		result := make(map[string]bool)
		for _, tok := range tokens {
			result[tok] = true
		}
		return result
	}

	tokens1 := extractTokens(err.Error())
	tokens2 := extractTokens(err2.Error())

	if !maps.Equal(tokens1, tokens2) {
		t.Errorf("error token sets differ:\n  got: %v\n  want: %v", tokens1, tokens2)
	}
}

func TestLoadNamespace_ReturnsCorrectConfig(t *testing.T) {
	// Reset cache before test
	lang.ClearCache()

	config := `
config : {
	log_level : "debug",
	log_format : "text"
}
other : {
	foo : "bar"
}`

	// Load config namespace
	loader := resolve("config")
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
	lang.ClearCache()

	config := `existing : { foo : "bar" }`

	// Load non-existent namespace
	loader := resolve("missing")
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
	lang.ClearCache()

	config := `config : { log_level : "debug" }`

	loader := resolve("config")
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
	lang.ClearCache()

	config := `test : { foo : "bar", baz : 42, nested : { a : 1, b : 2 } }`
	_, _ = lang.ParseString(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lang.ParseString(config)
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
		lang.ClearCache()
		b.StartTimer()

		_, err := lang.ParseString(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestGetOrParseConfig_ReadError verifies error handling for read failures.
func TestGetOrParseConfig_ReadError(t *testing.T) {
	// Create a reader that always fails
	errReader := &errorReader{err: bytes.ErrTooLarge}

	// ParseReader should return the error
	_, err := lang.ParseReader(errReader)
	if err == nil {
		t.Error("expected read error")
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
