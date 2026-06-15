package lang

import (
	"context"
	"fmt"
	"testing"
)

func TestGetNamespace(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		lookup string
		found  bool
	}{
		{
			name:   "single namespace found",
			input:  `x : 42`,
			lookup: "x",
			found:  true,
		},
		{
			name:   "single namespace not found",
			input:  `x : 42`,
			lookup: "y",
			found:  false,
		},
		{
			name:   "multiple namespaces found",
			input:  `a : 1; b : 2; c : 3; d : 4; e : 5`,
			lookup: "c",
			found:  true,
		},
		{
			name:   "multiple namespaces not found",
			input:  `a : 1; b : 2; c : 3`,
			lookup: "z",
			found:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			ns, ok := ast.GetNamespace(tt.lookup)
			if ok != tt.found {
				t.Errorf("expected found=%v, got found=%v", tt.found, ok)
			}
			if tt.found && ns == nil {
				t.Error("expected non-nil namespace when found=true")
			}
			if tt.found && ns.Name != tt.lookup {
				t.Errorf("expected namespace name %q, got %q", tt.lookup, ns.Name)
			}
		})
	}
}

func TestDefineNamespace(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		define   string
		expected int
	}{
		{
			name:     "add new namespace",
			initial:  `x : 1`,
			define:   "y",
			expected: 2,
		},
		{
			name:     "replace existing namespace",
			initial:  `x : 1`,
			define:   "x",
			expected: 1,
		},
		{
			name:     "add to empty AST",
			initial:  ``,
			define:   "z",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := ParseString(context.Background(), tt.initial)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			// Define a namespace
			ast.DefineNamespace(tt.define, nil, &Value{
				Kind:   KindExpr,
				Source: "42",
			})

			if len(ast.Namespaces) != tt.expected {
				t.Errorf("expected %d namespaces, got %d", tt.expected, len(ast.Namespaces))
			}

			// Verify it's findable via GetNamespace
			ns, ok := ast.GetNamespace(tt.define)
			if !ok {
				t.Errorf("defined namespace %q not found", tt.define)
			}
			if ns.Name != tt.define {
				t.Errorf("expected namespace name %q, got %q", tt.define, ns.Name)
			}

			// Verify index consistency
			if ast.index != nil && len(ast.index) != len(ast.Namespaces) {
				t.Errorf("index size mismatch: index=%d, namespaces=%d",
					len(ast.index), len(ast.Namespaces))
			}
		})
	}
}

func TestNamespaceIndexConsistency(t *testing.T) {
	// Create an AST with many namespaces
	var input string
	for i := 0; i < 100; i++ {
		if i > 0 {
			input += "; "
		}
		input += fmt.Sprintf("ns%d : %d", i, i)
	}

	ast, err := ParseString(context.Background(), input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Verify index was built
	if ast.index == nil {
		t.Fatal("expected index to be built after parsing")
	}

	// Verify index size matches namespace count
	if len(ast.index) != len(ast.Namespaces) {
		t.Errorf("index size mismatch: index=%d, namespaces=%d",
			len(ast.index), len(ast.Namespaces))
	}

	// Verify all namespaces are in the index
	for _, ns := range ast.Namespaces {
		indexed, ok := ast.index[ns.Name]
		if !ok {
			t.Errorf("namespace %q not found in index", ns.Name)
		}
		if indexed != ns {
			t.Errorf("index contains different pointer for namespace %q", ns.Name)
		}
	}

	// Verify GetNamespace returns correct results
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("ns%d", i)
		ns, ok := ast.GetNamespace(name)
		if !ok {
			t.Errorf("namespace %q not found", name)
		}
		if ns.Name != name {
			t.Errorf("expected namespace name %q, got %q", name, ns.Name)
		}
	}
}

// Benchmarks

func BenchmarkGetNamespace(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		// Generate input with 'size' namespaces
		var input string
		for i := 0; i < size; i++ {
			if i > 0 {
				input += "; "
			}
			input += fmt.Sprintf("ns%d : %d", i, i)
		}

		ast, err := ParseString(context.Background(), input)
		if err != nil {
			b.Fatalf("parse error: %v", err)
		}

		// Benchmark looking up namespace at different positions
		positions := []struct {
			name string
			idx  int
		}{
			{"first", 0},
			{"middle", size / 2},
			{"last", size - 1},
		}

		for _, pos := range positions {
			name := fmt.Sprintf("size=%d/position=%s", size, pos.name)
			lookupName := fmt.Sprintf("ns%d", pos.idx)

			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					ns, ok := ast.GetNamespace(lookupName)
					if !ok || ns == nil {
						b.Fatal("namespace not found")
					}
				}
			})
		}
	}
}

func TestMergeEntries_ExprLastWins(t *testing.T) {
	entries := []*Namespace{
		{Name: "x", Value: NewExpr("1")},
		{Name: "x", Value: NewExpr("2")},
	}

	merged := mergeEntries(entries)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(merged))
	}

	if merged[0].Value.Source != "2" {
		t.Errorf("expected source %q, got %q", "2", merged[0].Value.Source)
	}
}

func TestMergeEntries_BlockMerge(t *testing.T) {
	entries := []*Namespace{
		{
			Name: "config",
			Value: NewBlock(
				NewNamespace("a", nil, NewExpr("1")),
				NewNamespace("b", nil, NewExpr("2")),
			),
		},
		{
			Name: "config",
			Value: NewBlock(
				NewNamespace("a", nil, NewExpr("3")),
			),
		},
	}

	merged := mergeEntries(entries)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(merged))
	}

	block := merged[0].Value
	if block.Kind != KindBlock {
		t.Fatalf("expected block, got %v", block.Kind)
	}

	if len(block.Entries) != 2 {
		t.Fatalf("expected 2 block entries, got %d", len(block.Entries))
	}

	// "a" should be shadowed (value "3")
	if block.Entries[0].Name != "a" || block.Entries[0].Value.Source != "3" {
		t.Errorf("expected a=3, got %s=%s",
			block.Entries[0].Name, block.Entries[0].Value.Source)
	}

	// "b" should be preserved (value "2")
	if block.Entries[1].Name != "b" || block.Entries[1].Value.Source != "2" {
		t.Errorf("expected b=2, got %s=%s",
			block.Entries[1].Name, block.Entries[1].Value.Source)
	}
}

func TestMergeEntries_RecursiveBlockMerge(t *testing.T) {
	entries := []*Namespace{
		{
			Name: "outer",
			Value: NewBlock(
				NewNamespace("inner", nil, NewBlock(
					NewNamespace("x", nil, NewExpr("1")),
					NewNamespace("y", nil, NewExpr("2")),
				)),
			),
		},
		{
			Name: "outer",
			Value: NewBlock(
				NewNamespace("inner", nil, NewBlock(
					NewNamespace("x", nil, NewExpr("3")),
				)),
			),
		},
	}

	merged := mergeEntries(entries)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(merged))
	}

	inner := merged[0].Value.Entries[0]
	if inner.Name != "inner" {
		t.Fatalf("expected inner, got %s", inner.Name)
	}

	if len(inner.Value.Entries) != 2 {
		t.Fatalf("expected 2 inner entries, got %d", len(inner.Value.Entries))
	}

	// x should be shadowed
	if inner.Value.Entries[0].Value.Source != "3" {
		t.Errorf("expected x=3, got %s", inner.Value.Entries[0].Value.Source)
	}

	// y should be preserved
	if inner.Value.Entries[1].Value.Source != "2" {
		t.Errorf("expected y=2, got %s", inner.Value.Entries[1].Value.Source)
	}
}

func TestMergeEntries_MixedBlockExprLastWins(t *testing.T) {
	entries := []*Namespace{
		{
			Name:  "x",
			Value: NewBlock(NewNamespace("a", nil, NewExpr("1"))),
		},
		{
			Name:  "x",
			Value: NewExpr("42"),
		},
	}

	merged := mergeEntries(entries)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(merged))
	}

	if merged[0].Value.Kind != KindExpr {
		t.Errorf("expected KindExpr, got %v", merged[0].Value.Kind)
	}

	if merged[0].Value.Source != "42" {
		t.Errorf("expected source %q, got %q", "42", merged[0].Value.Source)
	}
}

func TestMergeEntries_PreservesOrder(t *testing.T) {
	entries := []*Namespace{
		{Name: "c", Value: NewExpr("3")},
		{Name: "a", Value: NewExpr("1")},
		{Name: "b", Value: NewExpr("2")},
		{Name: "a", Value: NewExpr("10")},
	}

	merged := mergeEntries(entries)
	if len(merged) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(merged))
	}

	// Order should follow first occurrence: c, a, b
	expected := []string{"c", "a", "b"}
	for i, name := range expected {
		if merged[i].Name != name {
			t.Errorf("position %d: expected %s, got %s", i, name, merged[i].Name)
		}
	}

	// "a" should have the last value
	if merged[1].Value.Source != "10" {
		t.Errorf("expected a=10, got %s", merged[1].Value.Source)
	}
}

func TestMergeEntries_NoDuplicates(t *testing.T) {
	entries := []*Namespace{
		{Name: "a", Value: NewExpr("1")},
		{Name: "b", Value: NewExpr("2")},
		{Name: "c", Value: NewExpr("3")},
	}

	merged := mergeEntries(entries)
	if len(merged) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(merged))
	}

	for i, ns := range entries {
		if merged[i] != ns {
			t.Errorf("position %d: expected same pointer", i)
		}
	}
}

func BenchmarkDefineNamespace(b *testing.B) {
	ast, err := ParseString(context.Background(), "x : 1; y : 2; z : 3")
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	value := &Value{
		Kind:   KindExpr,
		Source: "42",
	}

	b.Run("new", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Clone AST for each iteration to test adding new namespace
			testAST := &AST{
				Namespaces: make([]*Namespace, len(ast.Namespaces)),
				index:      make(map[string]*Namespace, len(ast.Namespaces)),
			}
			copy(testAST.Namespaces, ast.Namespaces)
			for k, v := range ast.index {
				testAST.index[k] = v
			}
			testAST.DefineNamespace("newns", nil, value)
		}
	})

	b.Run("replace", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Clone AST for each iteration
			testAST := &AST{
				Namespaces: make([]*Namespace, len(ast.Namespaces)),
				index:      make(map[string]*Namespace, len(ast.Namespaces)),
			}
			copy(testAST.Namespaces, ast.Namespaces)
			for k, v := range ast.index {
				testAST.index[k] = v
			}
			testAST.DefineNamespace("x", nil, value)
		}
	})
}
