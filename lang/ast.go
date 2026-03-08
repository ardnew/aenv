package lang

import (
	"context"
	"iter"
	"log/slog"

	"github.com/ardnew/aenv/log"
)

// AST is the root of the abstract syntax tree.
type AST struct {
	Namespaces []*Namespace
	index      map[string]*Namespace // O(1) namespace lookup index
	opts       options
	logger     log.Logger
}

// options holds AST-wide configuration.
type options struct {
	processEnv []string // process environment (defaults to os.Environ())
}

// Option is a functional option for AST configuration.
type Option func(*AST)

// WithLogger sets the logger for the AST.
func WithLogger(l log.Logger) Option {
	return func(a *AST) {
		a.logger = l
	}
}

// WithProcessEnv sets the process environment for env() function lookups.
// If not set, os.Environ() is used.
func WithProcessEnv(env []string) Option {
	return func(a *AST) {
		a.opts.processEnv = env
	}
}

// Namespace is a named definition with optional parameters and a value.
type Namespace struct {
	Name   string
	Params []Param
	Value  *Value
	Pos    Position
}

// Param describes a namespace parameter.
type Param struct {
	Name     string
	Variadic bool // true for ...name
}

// ValueKind distinguishes expressions from blocks.
type ValueKind int

const (
	KindExpr  ValueKind = iota // expr-lang expression
	KindBlock                  // { namespace* }
)

// String returns a string representation of the ValueKind.
func (k ValueKind) String() string {
	switch k {
	case KindExpr:
		return "Expr"
	case KindBlock:
		return "Block"
	default:
		return "Unknown"
	}
}

// Value is either a raw expression string or a block of namespaces.
type Value struct {
	Kind    ValueKind
	Source  string       // raw expression text (KindExpr only)
	Entries []*Namespace // named entries (KindBlock only)
	Pos     Position
}

// Position tracks source location for error messages.
type Position struct {
	Offset int // byte offset from start of input
	Line   int // line number (1-based)
	Column int // column number (1-based)
}

// GetNamespace returns the namespace with the given name, if it exists.
func (a *AST) GetNamespace(name string) (*Namespace, bool) {
	// Use index for O(1) lookup if available
	if a.index != nil {
		ns, ok := a.index[name]

		return ns, ok
	}

	// Fallback to linear search (e.g., during parsing before index is built)
	for _, ns := range a.Namespaces {
		if ns.Name == name {
			return ns, true
		}
	}

	return nil, false
}

// ValidateNamespaces checks that all non-parameterized top-level namespaces
// can be evaluated without errors. This is useful for catching configuration
// errors early, before interactive use.
func (a *AST) ValidateNamespaces(ctx context.Context, opts ...Option) error {
	for _, ns := range a.Namespaces {
		// Skip parameterized namespaces (they're validated at call time)
		if len(ns.Params) > 0 {
			continue
		}

		// Try to evaluate non-parameterized namespace
		_, err := a.EvaluateNamespace(ctx, ns.Name, nil, opts...)
		if err != nil {
			return ErrValidation.Wrap(err).
				With(slog.String("namespace", ns.Name))
		}
	}

	return nil
}

// All returns an iterator over all top-level namespaces.
func (a *AST) All() iter.Seq[*Namespace] {
	return func(yield func(*Namespace) bool) {
		for _, ns := range a.Namespaces {
			if !yield(ns) {
				return
			}
		}
	}
}

// RemoveNamespace removes all namespaces with the given name from the AST.
// It returns true if at least one namespace was removed.
func (a *AST) RemoveNamespace(name string) bool {
	filtered := a.Namespaces[:0]

	for _, ns := range a.Namespaces {
		if ns.Name != name {
			filtered = append(filtered, ns)
		}
	}

	removed := len(filtered) != len(a.Namespaces)
	a.Namespaces = filtered

	if removed && a.index != nil {
		delete(a.index, name)
	}

	return removed
}

// DefineNamespace adds or replaces a namespace in the AST.
func (a *AST) DefineNamespace(name string, params []Param, value *Value) {
	ns := &Namespace{
		Name:   name,
		Params: params,
		Value:  value,
	}

	// Check if namespace already exists
	for i, existing := range a.Namespaces {
		if existing.Name == name {
			// Replace existing
			a.Namespaces[i] = ns
			// Update index if it exists
			if a.index != nil {
				a.index[name] = ns
			}

			return
		}
	}

	// Add new
	a.Namespaces = append(a.Namespaces, ns)
	// Update index if it exists
	if a.index != nil {
		a.index[name] = ns
	}
}

// buildIndex creates the namespace lookup index for O(1) access.
// This is called after parsing is complete.
func (a *AST) buildIndex() {
	if len(a.Namespaces) == 0 {
		return
	}

	a.index = make(map[string]*Namespace, len(a.Namespaces))
	for _, ns := range a.Namespaces {
		a.index[ns.Name] = ns
	}
}

// resolvedNamespace returns the effective namespace definition for the given
// name by merging all block definitions. When multiple definitions exist and
// all have block values, their entries are merged recursively (later entries
// shadow earlier ones by name). Otherwise the last definition wins entirely.
func (a *AST) resolvedNamespace(name string) (*Namespace, bool) {
	var defs []*Namespace

	for _, ns := range a.Namespaces {
		if ns.Name == name {
			defs = append(defs, ns)
		}
	}

	if len(defs) == 0 {
		return nil, false
	}

	if len(defs) == 1 {
		return defs[0], true
	}

	merged := mergeEntries(defs)
	if len(merged) != 1 {
		// All share the same name, so mergeEntries produces exactly one.
		return defs[len(defs)-1], true
	}

	return merged[0], true
}

// mergeEntries takes a list of namespace entries that may contain duplicate
// names and returns a deduplicated list. The order of first occurrence is
// preserved. When two entries share a name:
//   - If both have block values, their entries are merged recursively.
//   - Otherwise the later definition replaces the earlier one entirely.
func mergeEntries(entries []*Namespace) []*Namespace {
	if len(entries) == 0 {
		return nil
	}

	entryMap := make(map[string]*Namespace, len(entries))
	order := make([]string, 0, len(entries))

	for _, entry := range entries {
		existing, seen := entryMap[entry.Name]
		if !seen {
			entryMap[entry.Name] = entry
			order = append(order, entry.Name)

			continue
		}

		// Both blocks: merge entries recursively.
		if existing.Value != nil && existing.Value.Kind == KindBlock &&
			entry.Value != nil && entry.Value.Kind == KindBlock {
			combined := make([]*Namespace, 0,
				len(existing.Value.Entries)+len(entry.Value.Entries))
			combined = append(combined, existing.Value.Entries...)
			combined = append(combined, entry.Value.Entries...)

			entryMap[entry.Name] = &Namespace{
				Name:   entry.Name,
				Params: entry.Params,
				Value: &Value{
					Kind:    KindBlock,
					Entries: mergeEntries(combined),
					Pos:     entry.Value.Pos,
				},
				Pos: entry.Pos,
			}

			continue
		}

		// Non-block or mixed: last definition wins.
		entryMap[entry.Name] = entry
	}

	result := make([]*Namespace, 0, len(order))
	for _, name := range order {
		result = append(result, entryMap[name])
	}

	return result
}
