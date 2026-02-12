package lang

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestAST_MarshalJSON(t *testing.T) {
	input := `test : { foo : 123, bar : "abc" }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	jsonData, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	// Parse the JSON to verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// Verify definition exists as object (tuple of all definitions)
	testNS, ok := result["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'test' to be object: %v", result)
	}

	// Verify foo and bar are direct keys
	if fooVal, ok := testNS["foo"].(float64); !ok || fooVal != 123 {
		t.Errorf("expected foo=123, got %v (type %T)", testNS["foo"], testNS["foo"])
	}

	if barVal, ok := testNS["bar"].(string); !ok || barVal != "abc" {
		t.Errorf("expected bar='abc', got %v (type %T)", testNS["bar"], testNS["bar"])
	}
}

func TestAST_MarshalJSON_WithArguments(t *testing.T) {
	input := `test region env : { foo : true }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	jsonData, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	testNS := result["test"].(map[string]interface{})

	// Verify parameters exist
	args, ok := testNS["(parameters)"].([]interface{})
	if !ok {
		t.Errorf("expected '(parameters)' in definition: %v", testNS)
	}

	if len(args) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(args))
	}

	// First parameter should be identifier "region"
	if arg0, ok := args[0].(string); !ok || arg0 != "region" {
		t.Errorf("expected first parameter to be 'region', got %v (type %T)", args[0], args[0])
	}

	// Second parameter should be identifier "env"
	if arg1, ok := args[1].(string); !ok || arg1 != "env" {
		t.Errorf("expected second parameter to be 'env', got %v (type %T)", args[1], args[1])
	}

	// Since value is a map (object), it should be flattened alongside (parameters)
	// Value should be a tuple-as-object with foo:true directly in testNS
	if fooVal, ok := testNS["foo"].(bool); !ok || fooVal != true {
		t.Errorf("expected foo=true, got %v (type %T)", testNS["foo"], testNS["foo"])
	}
}

func TestAST_MarshalJSON_WithList(t *testing.T) {
	input := `test : { items : {1, 2, 3} }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	jsonData, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// test is an object (tuple of all definitions)
	testNS, ok := result["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'test' to be object: %v", result)
	}

	// items should be a direct key with array value
	items, ok := testNS["items"].([]interface{})
	if !ok {
		t.Fatalf("expected 'items' to be array: %v", testNS)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	for i, expected := range []float64{1, 2, 3} {
		if val, ok := items[i].(float64); !ok || val != expected {
			t.Errorf("expected items[%d]=%v, got %v (type %T)", i, expected, items[i], items[i])
		}
	}
}

func TestAST_MarshalJSON_WithNestedTuple(t *testing.T) {
	input := `test : { config : { port : 8080 } }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	jsonData, err := json.Marshal(ast)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// test is an object
	testNS, ok := result["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'test' to be object: %v", result)
	}

	// config should be a nested object
	configObj, ok := testNS["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'config' to be object: %v", testNS)
	}

	// port should be directly in config
	if port, ok := configObj["port"].(float64); !ok || port != 8080 {
		t.Errorf("expected port=8080, got %v (type %T)", configObj["port"], configObj["port"])
	}
}

func TestAST_MarshalYAML(t *testing.T) {
	input := `test : { foo : 123, bar : "abc" }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	yamlData, err := yaml.Marshal(ast.ToMap())
	if err != nil {
		t.Fatalf("YAML marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		t.Fatalf("YAML unmarshal error: %v", err)
	}

	// Verify definition exists as object (tuple of all definitions)
	testNS, ok := result["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'test' to be object: %v", result)
	}

	// Verify foo and bar are direct keys
	// YAML may return int, int64, or uint64 depending on the implementation
	fooVal := testNS["foo"]
	var foo int64
	switch v := fooVal.(type) {
	case int:
		foo = int64(v)
	case int64:
		foo = v
	case uint64:
		foo = int64(v)
	default:
		t.Errorf("expected foo to be int, got %T: %v", fooVal, fooVal)
	}
	if foo != 123 {
		t.Errorf("expected foo=123, got %v", foo)
	}

	if barVal, ok := testNS["bar"].(string); !ok || barVal != "abc" {
		t.Errorf("expected bar='abc', got %v (type %T)", testNS["bar"], testNS["bar"])
	}

	// Verify YAML structure
	yamlStr := string(yamlData)
	if !strings.Contains(yamlStr, "test:") {
		t.Errorf("YAML should contain 'test:' but got: %s", yamlStr)
	}
	if !strings.Contains(yamlStr, "foo:") {
		t.Errorf("YAML should contain 'foo:' but got: %s", yamlStr)
	}
}

func TestAST_MarshalYAML_WithArguments(t *testing.T) {
	input := `test region env : { greeting : "hello" }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	yamlData, err := yaml.Marshal(ast.ToMap())
	if err != nil {
		t.Fatalf("YAML marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		t.Fatalf("YAML unmarshal error: %v", err)
	}

	testNS := result["test"].(map[string]interface{})

	// Verify parameters exist
	args, ok := testNS["(parameters)"].([]interface{})
	if !ok {
		t.Errorf("expected '(parameters)' in definition: %v", testNS)
	}

	if len(args) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(args))
	}

	// Since value is a map (object), it should be flattened alongside (parameters)
	// Value should be a tuple-as-object with greeting:"hello" directly in testNS
	if greetingVal, ok := testNS["greeting"].(string); !ok || greetingVal != "hello" {
		t.Errorf("expected greeting='hello', got %v (type %T)", testNS["greeting"], testNS["greeting"])
	}

	// Verify YAML contains parameters
	yamlStr := string(yamlData)
	if !strings.Contains(yamlStr, "(parameters):") {
		t.Errorf("YAML should contain '(parameters):' but got: %s", yamlStr)
	}
}

func TestAST_MarshalYAML_WithList(t *testing.T) {
	input := `test : { items : {1, 2, 3} }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	yamlData, err := yaml.Marshal(ast.ToMap())
	if err != nil {
		t.Fatalf("YAML marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		t.Fatalf("YAML unmarshal error: %v", err)
	}

	// test is an object (tuple of all definitions)
	testNS, ok := result["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'test' to be object: %v", result)
	}

	// items should be a direct key with array value
	items, ok := testNS["items"].([]interface{})
	if !ok {
		t.Fatalf("expected 'items' to be array: %v", testNS)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// YAML may return int, int64, or uint64 depending on the implementation
	for i, expected := range []int64{1, 2, 3} {
		var val int64
		switch v := items[i].(type) {
		case int:
			val = int64(v)
		case int64:
			val = v
		case uint64:
			val = int64(v)
		default:
			t.Errorf("expected items[%d] to be int, got %T: %v", i, items[i], items[i])
			continue
		}
		if val != expected {
			t.Errorf("expected items[%d]=%v, got %v", i, expected, val)
		}
	}

	// Verify YAML contains expected keys
	yamlStr := string(yamlData)
	if !strings.Contains(yamlStr, "items") {
		t.Errorf("YAML should contain 'items' but got: %s", yamlStr)
	}
}

func TestAST_MarshalYAML_WithNestedTuple(t *testing.T) {
	input := `test : { config : { port : 8080 } }`

	ast, err := ParseString(t.Context(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	yamlData, err := yaml.Marshal(ast.ToMap())
	if err != nil {
		t.Fatalf("YAML marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		t.Fatalf("YAML unmarshal error: %v", err)
	}

	// test is an object
	testNS, ok := result["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'test' to be object: %v", result)
	}

	// config should be a nested object
	configObj, ok := testNS["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'config' to be object: %v", testNS)
	}

	// YAML may return int, int64, or uint64 depending on the implementation
	portVal := configObj["port"]
	var port int64
	switch v := portVal.(type) {
	case int:
		port = int64(v)
	case int64:
		port = v
	case uint64:
		port = int64(v)
	default:
		t.Errorf("expected port value to be int, got %T: %v", portVal, portVal)
		return
	}

	if port != 8080 {
		t.Errorf("expected port value=8080, got %v", port)
	}

	// Verify YAML contains expected structure
	yamlStr := string(yamlData)
	if !strings.Contains(yamlStr, "config:") {
		t.Errorf("YAML should contain 'config:' but got: %s", yamlStr)
	}
	if !strings.Contains(yamlStr, "port:") {
		t.Errorf("YAML should contain 'port:' but got: %s", yamlStr)
	}
}
