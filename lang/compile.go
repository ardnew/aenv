package lang

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/expr-lang/expr"

	"github.com/ardnew/aenv/log"
)

// scope tracks the namespaces visible at a given nesting level.
type scope struct {
	ns     []*Namespace
	params []*Value
}

// CompileExprs walks the AST and compiles all TypeExpr values
// using expr-lang. Each expression gets an environment built from
// its position in the AST.
func (ast *AST) CompileExprs(ctx context.Context) error {
	ast.logger.TraceContext(
		ctx,
		"compile start",
		slog.Int("namespace_count", len(ast.Namespaces)),
		slog.Bool("compile_enabled", ast.opts.compileExprs),
	)

	if !ast.opts.compileExprs {
		return nil
	}

	processEnv := buildProcessEnvMap(ast.opts.processEnv)

	topScope := scope{
		ns:     ast.Namespaces,
		params: nil,
	}

	for _, ns := range ast.Namespaces {
		err := compileNamespace(
			ctx, ns, processEnv, []scope{topScope}, ast,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// compileNamespace compiles all expr values within a namespace.
// scopes provides the chain of visible scopes from outermost to
// innermost.
func compileNamespace(
	ctx context.Context,
	ns *Namespace,
	processEnv map[string]string,
	scopes []scope,
	root *AST,
) error {
	if ns != nil && ns.Identifier != nil {
		root.logger.TraceContext(
			ctx,
			"compile namespace",
			slog.String("identifier", ns.Identifier.LiteralString()),
			slog.Int("scope_count", len(scopes)),
		)
	}

	if ns.Value == nil {
		return nil
	}

	nsScope := scope{
		ns:     nil,
		params: ns.Parameters,
	}
	innerScopes := append(scopes, nsScope)

	return compileValue(ctx, ns.Value, processEnv, innerScopes, root)
}

// compileValue dispatches compilation based on value type.
func compileValue(
	ctx context.Context,
	v *Value,
	processEnv map[string]string,
	scopes []scope,
	root *AST,
) error {
	if v != nil {
		root.logger.TraceContext(
			ctx,
			"compile dispatch",
			slog.String("value_type", v.Type.String()),
		)
	}

	switch v.Type {
	case TypeExpr:
		return compileExpr(ctx, v, processEnv, scopes, root)

	case TypeTuple:
		return compileTuple(ctx, v, processEnv, scopes, root)

	case TypeNamespace:
		if v.Namespace == nil {
			return nil
		}

		return compileNamespace(ctx, v.Namespace, processEnv, scopes, root)

	case TypeIdentifier, TypeBoolean, TypeNumber, TypeString:
		return nil

	default:
		return nil
	}
}

// compileTuple compiles all values within a tuple.
func compileTuple(
	ctx context.Context,
	v *Value,
	processEnv map[string]string,
	scopes []scope,
	root *AST,
) error {
	if v.Tuple == nil {
		return nil
	}

	var tupleNS []*Namespace

	for _, val := range v.Tuple.Values {
		if val.Type == TypeNamespace && val.Namespace != nil {
			tupleNS = append(tupleNS, val.Namespace)
		}
	}

	tupleScope := scope{
		ns:     tupleNS,
		params: nil,
	}
	innerScopes := append(scopes, tupleScope)

	for _, val := range v.Tuple.Values {
		if val.Type == TypeNamespace && val.Namespace != nil {
			err := compileNamespace(
				ctx, val.Namespace, processEnv, innerScopes, root,
			)
			if err != nil {
				return err
			}
		} else {
			err := compileValue(ctx, val, processEnv, innerScopes, root)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// compileExpr compiles a single expr-lang expression and stores the
// resulting program on the Value.
//
// If the expression uses parameters (which have unknown types at compile time),
// compilation may fail. In this case, we leave Program == nil and defer to
// lazy compilation at evaluation time when actual parameter types are known.
func compileExpr(
	ctx context.Context,
	v *Value,
	processEnv map[string]string,
	scopes []scope,
	root *AST,
) error {
	source := v.ExprSource()

	// Skip empty expressions — expr-lang cannot compile them
	if source == "" {
		return nil
	}

	env := buildExprEnv(ctx, scopes, processEnv, root.logger)

	// Check if there are any parameters in scope (typed as any(nil))
	hasParams := scopesHaveParams(scopes)

	root.logger.TraceContext(
		ctx,
		"compile expr",
		slog.String("source", source),
		slog.Any("env_keys", sortedKeys(env)),
		slog.Bool("has_params", hasParams),
	)

	patcher := &hyphenPatcher{
		namespaces: root.Namespaces,
		env:        env,
		logger:     root.logger,
	}

	program, err := expr.Compile(source, expr.Env(env), expr.Patch(patcher))
	if err != nil {
		// If we have parameters, defer to lazy compilation at runtime
		// when we'll know the actual parameter types
		if hasParams {
			v.Program = nil

			root.logger.TraceContext(
				ctx,
				"compile result",
				slog.Bool("deferred", true),
			)

			return nil
		}

		root.logger.TraceContext(
			ctx,
			"compile result",
			slog.Bool("compiled", false),
		)

		return ErrExprCompile.Wrap(err).
			With(slog.String("source", source))
	}

	v.Program = program

	root.logger.TraceContext(
		ctx,
		"compile result",
		slog.Bool("compiled", true),
	)

	return nil
}

// scopesHaveParams returns true if any scope contains parameters.
func scopesHaveParams(scopes []scope) bool {
	for _, s := range scopes {
		if len(s.params) > 0 {
			return true
		}
	}

	return false
}

// buildExprEnv constructs the environment map for expr compilation.
// It walks the scope chain from innermost to outermost, collecting
// all visible namespaces and parameters.
// Namespaces with parameters are added as callable functions.
func buildExprEnv(
	ctx context.Context,
	scopes []scope,
	processEnv map[string]string,
	logger log.Logger,
) map[string]any {
	env := make(map[string]any)

	for i := len(scopes) - 1; i >= 0; i-- {
		s := scopes[i]

		for _, ns := range s.ns {
			name := ns.Identifier.LiteralString()
			if _, exists := env[name]; !exists {
				if len(ns.Parameters) > 0 {
					// Add as a function type exemplar for type checking.
					// The function body is never called during compilation,
					// it only provides type information to expr-lang.
					env[name] = func(...any) (any, error) { return 0, nil }
				} else {
					env[name] = inferTypeExemplar(ns.Value)
				}
			}
		}

		for _, param := range s.params {
			if param.Token != nil {
				pName := param.Token.LiteralString()
				if _, exists := env[pName]; !exists {
					env[pName] = any(nil)
				}
			}
		}
	}

	env["env"] = envFunc(processEnv)

	logger.TraceContext(
		ctx,
		"build env",
		slog.Int("scope_count", len(scopes)),
		slog.Any("env_keys", sortedKeys(env)),
	)

	return env
}

// inferTypeExemplar maps an aenv Value type to a Go type exemplar
// for expr-lang compilation.
func inferTypeExemplar(v *Value) any {
	if v == nil {
		return any(nil)
	}

	switch v.Type {
	case TypeBoolean:
		return false
	case TypeNumber:
		return inferNumberExemplar(v)
	case TypeString:
		return ""
	case TypeIdentifier, TypeExpr:
		return any(nil)
	case TypeTuple:
		return inferTupleExemplar(v)
	case TypeNamespace:
		return inferNamespaceExemplar(v)
	default:
		return any(nil)
	}
}

// inferNumberExemplar returns int64 or float64 based on the literal.
func inferNumberExemplar(v *Value) any {
	if v.Token != nil {
		lit := v.Token.LiteralString()
		if strings.ContainsAny(lit, ".eE") {
			return float64(0)
		}
	}

	return int64(0)
}

// inferTupleExemplar returns map[string]any for all-namespace
// tuples, []any otherwise.
func inferTupleExemplar(v *Value) any {
	if v.Tuple == nil {
		return any(nil)
	}

	allNamespaces := true

	for _, val := range v.Tuple.Values {
		if val.Type != TypeNamespace {
			allNamespaces = false

			break
		}
	}

	if allNamespaces && len(v.Tuple.Values) > 0 {
		return map[string]any{}
	}

	return []any{}
}

// inferNamespaceExemplar returns a function exemplar for
// parametric namespaces or recurses for the value.
func inferNamespaceExemplar(v *Value) any {
	if v.Namespace == nil || v.Namespace.Value == nil {
		return any(nil)
	}

	if len(v.Namespace.Parameters) > 0 {
		return func(...any) any { return nil }
	}

	return inferTypeExemplar(v.Namespace.Value)
}

// buildProcessEnvMap converts a "KEY=VALUE" string slice to a map.
// If envList is nil, os.Environ() is used.
func buildProcessEnvMap(envList []string, keyVal ...string) map[string]string {
	envList = append(envList, keyVal...)
	if len(envList) == 0 {
		envList = os.Environ()
	}

	result := make(map[string]string, len(envList))

	for _, entry := range envList {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}

	return result
}

// envFunc returns the built-in env() function that provides
// process environment access to expr programs.
func envFunc(processEnv map[string]string) func(string) string {
	return func(key string) string {
		return processEnv[key]
	}
}
