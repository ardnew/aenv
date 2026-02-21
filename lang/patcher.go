package lang

import (
	"log/slog"

	"github.com/expr-lang/expr/ast"

	"github.com/ardnew/aenv/log"
)

// hyphenPatcher reconstructs hyphenated identifiers from BinaryNode("-")
// subtraction chains created by expr-lang's parser.
//
// The aenv DSL allows hyphens in identifiers (e.g., "log-pretty"), but
// expr-lang parses them as subtraction. This visitor detects subtraction
// chains and, when the combined name exists in the environment or namespace
// tree, patches the node to a single identifier or member access.
type hyphenPatcher struct {
	namespaces []*Namespace   // AST namespaces for resolving member paths
	env        map[string]any // Flat environment for checking top-level names
	logger     log.Logger
}

// Visit implements ast.Visitor for hyphenPatcher.
func (p *hyphenPatcher) Visit(node *ast.Node) {
	binNode, ok := (*node).(*ast.BinaryNode)
	if !ok || binNode.Operator != "-" {
		return
	}

	// Right side must be an identifier (the segment after the hyphen).
	rightIdent, ok := binNode.Right.(*ast.IdentifierNode)
	if !ok {
		return
	}

	switch left := binNode.Left.(type) {
	case *ast.MemberNode:
		p.patchMember(node, left, rightIdent)

	case *ast.BinaryNode:
		if left.Operator == "-" {
			p.patchChain(node, left, rightIdent)
		}

	case *ast.IdentifierNode:
		p.patchTopLevel(node, left, rightIdent)
	}
}

// patchMember handles MemberNode(base, "prop") - IdentNode("name")
// and rewrites it to MemberNode(base, "prop-name") when valid.
func (p *hyphenPatcher) patchMember(
	node *ast.Node,
	left *ast.MemberNode,
	right *ast.IdentifierNode,
) {
	prop, ok := left.Property.(*ast.StringNode)
	if !ok {
		return
	}

	combined := prop.Value + "-" + right.Value

	basePath, ok := extractMemberPath(left.Node)
	if !ok {
		return
	}

	if !p.hasChild(basePath, combined) {
		return
	}

	ast.Patch(node, &ast.MemberNode{
		Node:     left.Node,
		Property: &ast.StringNode{Value: combined},
		Optional: false,
		Method:   false,
	})

	p.logger.Trace("patch hyphenated",
		slog.String("combined_name", combined),
		slog.String("patch_type", "member"))
}

// patchChain handles nested BinaryNode("-") chains where the inner node
// was not patched (e.g., because only the full chain matches a name).
func (p *hyphenPatcher) patchChain(
	node *ast.Node,
	left *ast.BinaryNode,
	right *ast.IdentifierNode,
) {
	base, property, ok := extractHyphenChain(left)
	if !ok {
		return
	}

	combined := property + "-" + right.Value

	if base == nil {
		// Top-level identifier chain: e.g., log-pretty-print
		if p.hasTopLevel(combined) {
			ast.Patch(node, &ast.IdentifierNode{Value: combined})
			p.logger.Trace("patch hyphenated",
				slog.String("combined_name", combined),
				slog.String("patch_type", "chain"))
		}

		return
	}

	// Member access chain: e.g., config.log-pretty-print
	basePath, ok := extractMemberPath(base)
	if !ok {
		return
	}

	if p.hasChild(basePath, combined) {
		ast.Patch(node, &ast.MemberNode{
			Node:     base,
			Property: &ast.StringNode{Value: combined},
			Optional: false,
			Method:   false,
		})
		p.logger.Trace("patch hyphenated",
			slog.String("combined_name", combined),
			slog.String("patch_type", "chain"))
	}
}

// patchTopLevel handles IdentNode("a") - IdentNode("b") and rewrites
// to IdentNode("a-b") when "a-b" exists in the environment.
func (p *hyphenPatcher) patchTopLevel(
	node *ast.Node,
	left *ast.IdentifierNode,
	right *ast.IdentifierNode,
) {
	combined := left.Value + "-" + right.Value
	if p.hasTopLevel(combined) {
		ast.Patch(node, &ast.IdentifierNode{Value: combined})
		p.logger.Trace("patch hyphenated",
			slog.String("combined_name", combined),
			slog.String("patch_type", "top-level"))
	}
}

// extractHyphenChain recursively walks unpatched BinaryNode("-") chains
// to extract the base node and accumulated hyphenated property string.
func extractHyphenChain(
	bin *ast.BinaryNode,
) (base ast.Node, property string, ok bool) {
	if bin.Operator != "-" {
		return nil, "", false
	}

	rightIdent, ok := bin.Right.(*ast.IdentifierNode)
	if !ok {
		return nil, "", false
	}

	switch left := bin.Left.(type) {
	case *ast.MemberNode:
		prop, ok := left.Property.(*ast.StringNode)
		if !ok {
			return nil, "", false
		}

		return left.Node, prop.Value + "-" + rightIdent.Value, true

	case *ast.BinaryNode:
		if left.Operator != "-" {
			return nil, "", false
		}

		innerBase, innerProp, ok := extractHyphenChain(left)
		if !ok {
			return nil, "", false
		}

		return innerBase, innerProp + "-" + rightIdent.Value, true

	case *ast.IdentifierNode:
		// Top-level chain: ident - ident
		return nil, left.Value + "-" + rightIdent.Value, true

	default:
		return nil, "", false
	}
}

// extractMemberPath walks a MemberNode chain to produce path segments.
func extractMemberPath(node ast.Node) ([]string, bool) {
	switch n := node.(type) {
	case *ast.IdentifierNode:
		return []string{n.Value}, true

	case *ast.MemberNode:
		prop, ok := n.Property.(*ast.StringNode)
		if !ok {
			return nil, false
		}

		base, ok := extractMemberPath(n.Node)
		if !ok {
			return nil, false
		}

		return append(base, prop.Value), true

	default:
		return nil, false
	}
}

// hasTopLevel checks whether the combined name exists in the environment.
func (p *hyphenPatcher) hasTopLevel(name string) bool {
	_, ok := p.env[name]

	return ok
}

// hasChild checks whether the namespace at basePath has a child with
// the given name.
func (p *hyphenPatcher) hasChild(basePath []string, childName string) bool {
	val := p.resolvePath(basePath)
	if val == nil {
		return false
	}

	return findChildValue(val, childName) != nil
}

// resolvePath walks the namespace tree to find the Value at the given path.
func (p *hyphenPatcher) resolvePath(segments []string) *Value {
	if len(segments) == 0 {
		return nil
	}

	val := p.resolveBase(segments[0])

	for _, seg := range segments[1:] {
		val = findChildValue(val, seg)
		if val == nil {
			return nil
		}
	}

	return val
}

// resolveBase finds a top-level namespace by name.
func (p *hyphenPatcher) resolveBase(name string) *Value {
	for _, ns := range p.namespaces {
		if ns.Name == name {
			return ns.Value
		}
	}

	return nil
}

// findChildValue looks up a child namespace by name within a block value.
func findChildValue(v *Value, name string) *Value {
	if v == nil || v.Kind != KindBlock {
		return nil
	}

	for _, child := range v.Entries {
		if child.Name == name {
			return child.Value
		}
	}

	return nil
}

// resultTypeName returns a string representation of a value's type.
func resultTypeName(v any) string {
	if v == nil {
		return "nil"
	}

	switch v.(type) {
	case bool:
		return "bool"
	case int, int8, int16, int32, int64:
		return "int"
	case uint, uint8, uint16, uint32, uint64:
		return "uint"
	case float32, float64:
		return "float"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "map"
	default:
		return "unknown"
	}
}
