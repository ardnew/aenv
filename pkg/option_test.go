package pkg

import (
	"testing"
)

func TestMake(t *testing.T) {
	// Test Make with no options
	result := Make[int]()
	expected := 0 // zero value for int
	if result != expected {
		t.Errorf("Make() = %d, want %d", result, expected)
	}

	// Test Make with single option
	addFive := func(x int) int { return x + 5 }
	result = Make(addFive)
	expected = 5
	if result != expected {
		t.Errorf("Make(addFive) = %d, want %d", result, expected)
	}

	// Test Make with multiple options
	addOne := func(x int) int { return x + 1 }
	addTwo := func(x int) int { return x + 2 }
	result = Make(addOne, addTwo)
	expected = 3 // 0 + 1 + 2
	if result != expected {
		t.Errorf("Make(addOne, addTwo) = %d, want %d", result, expected)
	}
}

func TestMakeWithString(t *testing.T) {
	// Test Make with string type
	prefix := func(s string) string { return "prefix_" + s }
	suffix := func(s string) string { return s + "_suffix" }

	result := Make(prefix, suffix)
	expected := "prefix__suffix"
	if result != expected {
		t.Errorf("Make(prefix, suffix) = %q, want %q", result, expected)
	}
}

func TestWrap(t *testing.T) {
	// Test Wrap with no options
	initial := 10
	result := Wrap(initial)
	if result != initial {
		t.Errorf("Wrap(%d) = %d, want %d", initial, result, initial)
	}

	// Test Wrap with single option
	multiply2 := func(x int) int { return x * 2 }
	result = Wrap(initial, multiply2)
	expected := 20
	if result != expected {
		t.Errorf("Wrap(%d, multiply2) = %d, want %d", initial, result, expected)
	}

	// Test Wrap with multiple options
	add10 := func(x int) int { return x + 10 }
	divide2 := func(x int) int { return x / 2 }
	result = Wrap(initial, add10, divide2)
	expected = 10 // (10 + 10) / 2
	if result != expected {
		t.Errorf(
			"Wrap(%d, add10, divide2) = %d, want %d",
			initial,
			result,
			expected,
		)
	}
}

func TestWrapWithStruct(t *testing.T) {
	type testStruct struct {
		Name  string
		Value int
	}

	setName := func(ts testStruct) testStruct {
		ts.Name = "test"
		return ts
	}

	setValue := func(ts testStruct) testStruct {
		ts.Value = 42
		return ts
	}

	initial := testStruct{}
	result := Wrap(initial, setName, setValue)

	if result.Name != "test" {
		t.Errorf("Expected Name to be 'test', got %q", result.Name)
	}
	if result.Value != 42 {
		t.Errorf("Expected Value to be 42, got %d", result.Value)
	}
}

func TestOptionChaining(t *testing.T) {
	// Test that options are applied in order
	op1 := func(s []string) []string {
		return append(s, "op1")
	}

	op2 := func(s []string) []string {
		return append(s, "op2")
	}

	op3 := func(s []string) []string {
		return append(s, "op3")
	}

	result := Make(op1, op2, op3)
	expected := []string{"op1", "op2", "op3"}

	if len(result) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(result))
	}

	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected result[%d] to be %q, got %q", i, v, result[i])
		}
	}
}
