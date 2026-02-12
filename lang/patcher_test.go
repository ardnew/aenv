package lang

import (
	"testing"

	exprAst "github.com/expr-lang/expr/ast"
)

func TestExtractHyphenChain(t *testing.T) {
	tests := []struct {
		name     string
		bin      *exprAst.BinaryNode
		wantBase bool // true = member base, false = nil (top-level)
		wantProp string
		wantOK   bool
	}{
		{
			name: "member_simple",
			bin: &exprAst.BinaryNode{
				Operator: "-",
				Left: &exprAst.MemberNode{
					Node:     &exprAst.IdentifierNode{Value: "config"},
					Property: &exprAst.StringNode{Value: "log"},
				},
				Right: &exprAst.IdentifierNode{Value: "pretty"},
			},
			wantBase: true,
			wantProp: "log-pretty",
			wantOK:   true,
		},
		{
			name: "top_level_simple",
			bin: &exprAst.BinaryNode{
				Operator: "-",
				Left:     &exprAst.IdentifierNode{Value: "log"},
				Right:    &exprAst.IdentifierNode{Value: "pretty"},
			},
			wantBase: false,
			wantProp: "log-pretty",
			wantOK:   true,
		},
		{
			name: "chained_three_segments",
			bin: &exprAst.BinaryNode{
				Operator: "-",
				Left: &exprAst.BinaryNode{
					Operator: "-",
					Left: &exprAst.MemberNode{
						Node:     &exprAst.IdentifierNode{Value: "config"},
						Property: &exprAst.StringNode{Value: "log"},
					},
					Right: &exprAst.IdentifierNode{Value: "pretty"},
				},
				Right: &exprAst.IdentifierNode{Value: "print"},
			},
			wantBase: true,
			wantProp: "log-pretty-print",
			wantOK:   true,
		},
		{
			name: "top_level_three_segments",
			bin: &exprAst.BinaryNode{
				Operator: "-",
				Left: &exprAst.BinaryNode{
					Operator: "-",
					Left:     &exprAst.IdentifierNode{Value: "a"},
					Right:    &exprAst.IdentifierNode{Value: "b"},
				},
				Right: &exprAst.IdentifierNode{Value: "c"},
			},
			wantBase: false,
			wantProp: "a-b-c",
			wantOK:   true,
		},
		{
			name: "right_not_ident",
			bin: &exprAst.BinaryNode{
				Operator: "-",
				Left:     &exprAst.IdentifierNode{Value: "a"},
				Right:    &exprAst.IntegerNode{Value: 5},
			},
			wantOK: false,
		},
		{
			name: "wrong_operator",
			bin: &exprAst.BinaryNode{
				Operator: "+",
				Left:     &exprAst.IdentifierNode{Value: "a"},
				Right:    &exprAst.IdentifierNode{Value: "b"},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, prop, ok := extractHyphenChain(tt.bin)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if !ok {
				return
			}

			if prop != tt.wantProp {
				t.Errorf("property = %q, want %q", prop, tt.wantProp)
			}

			if tt.wantBase && base == nil {
				t.Error("expected non-nil base, got nil")
			}

			if !tt.wantBase && base != nil {
				t.Error("expected nil base, got non-nil")
			}
		})
	}
}

func TestExtractMemberPath(t *testing.T) {
	tests := []struct {
		name     string
		node     exprAst.Node
		wantPath []string
		wantOK   bool
	}{
		{
			name:     "single_ident",
			node:     &exprAst.IdentifierNode{Value: "config"},
			wantPath: []string{"config"},
			wantOK:   true,
		},
		{
			name: "two_segments",
			node: &exprAst.MemberNode{
				Node:     &exprAst.IdentifierNode{Value: "a"},
				Property: &exprAst.StringNode{Value: "b"},
			},
			wantPath: []string{"a", "b"},
			wantOK:   true,
		},
		{
			name: "three_segments",
			node: &exprAst.MemberNode{
				Node: &exprAst.MemberNode{
					Node:     &exprAst.IdentifierNode{Value: "a"},
					Property: &exprAst.StringNode{Value: "b"},
				},
				Property: &exprAst.StringNode{Value: "c"},
			},
			wantPath: []string{"a", "b", "c"},
			wantOK:   true,
		},
		{
			name:   "not_ident_or_member",
			node:   &exprAst.IntegerNode{Value: 5},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := extractMemberPath(tt.node)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if !ok {
				return
			}

			if len(path) != len(tt.wantPath) {
				t.Fatalf("path len = %d, want %d", len(path), len(tt.wantPath))
			}

			for i := range path {
				if path[i] != tt.wantPath[i] {
					t.Errorf(
						"path[%d] = %q, want %q",
						i, path[i], tt.wantPath[i],
					)
				}
			}
		})
	}
}

func TestFindChildValue(t *testing.T) {
	// Build a simple namespace tree: config { log-pretty : true, port : 8080 }
	configVal := &Value{
		Type: TypeTuple,
		Tuple: &Tuple{
			Values: []*Value{
				{
					Type: TypeNamespace,
					Namespace: &Namespace{
						Identifier: newToken("identifier", "log-pretty"),
						Value:      &Value{Type: TypeBoolean},
					},
				},
				{
					Type: TypeNamespace,
					Namespace: &Namespace{
						Identifier: newToken("identifier", "port"),
						Value:      &Value{Type: TypeNumber},
					},
				},
			},
		},
	}

	if v := findChildValue(configVal, "log-pretty"); v == nil {
		t.Error("expected to find log-pretty child, got nil")
	}

	if v := findChildValue(configVal, "port"); v == nil {
		t.Error("expected to find port child, got nil")
	}

	if v := findChildValue(configVal, "nonexistent"); v != nil {
		t.Error("expected nil for nonexistent child, got non-nil")
	}

	if v := findChildValue(nil, "anything"); v != nil {
		t.Error("expected nil for nil value, got non-nil")
	}
}
