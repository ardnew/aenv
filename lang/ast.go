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

// DefineNamespace adds or replaces a namespace in the AST.
func (a *AST) DefineNamespace(name string, params []Param, value *Value) {
	// Check if namespace already exists
	for i, ns := range a.Namespaces {
		if ns.Name == name {
			// Replace existing
			a.Namespaces[i] = &Namespace{
				Name:   name,
				Params: params,
				Value:  value,
			}

			return
		}
	}
	// Add new
	a.Namespaces = append(a.Namespaces, &Namespace{
		Name:   name,
		Params: params,
		Value:  value,
	})
}
