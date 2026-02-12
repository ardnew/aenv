package lang

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/ardnew/aenv/log"
)

// EvaluateNamespace evaluates a namespace with the given parameter bindings.
// The args slice provides values for each parameter in order.
// Returns the fully evaluated result.
//
// Options are applied as overrides to ast.opts for this evaluation only,
// enabling thread-safe concurrent evaluations with different processEnv.
func (ast *AST) EvaluateNamespace(
	ctx context.Context,
	name string,
	args []string,
	opts ...Option,
) (any, error) {
	// Copy ast.opts locally for thread-safe concurrent evaluation
	local := ast.opts
	logger := ast.logger

	for _, opt := range opts {
		// Apply option to a temporary AST to extract the setting
		var temp AST

		temp.opts = local
		temp.logger = logger
		opt(&temp)
		local = temp.opts
		logger = temp.logger
	}

	logger.TraceContext(
		ctx,
		"eval namespace",
		slog.String("name", name),
		slog.Int("arg_count", len(args)),
	)

	ns, found := ast.GetNamespace(name)
	if !found {
		return nil, ErrNotDefined.
			With(slog.String("name", name))
	}

	if len(args) != len(ns.Parameters) {
		return nil, ErrParameterCount.
			With(
				slog.String("name", name),
				slog.Int("expected", len(ns.Parameters)),
				slog.Int("got", len(args)),
			)
	}

	processEnv := buildProcessEnvMap(local.processEnv)

	// Build parameter bindings map by parsing each argument as a Value
	params := make(map[string]*Value, len(args))

	for i, param := range ns.Parameters {
		if param.Token != nil {
			paramName := param.Token.LiteralString()
			params[paramName] = ast.parseArgToValue(ctx, args[i])
		}
	}

	logger.TraceContext(
		ctx,
		"param bindings",
		slog.Any("params", sortedKeys(params)),
	)

	// Build evaluation context from AST scope
	ectx := &evalContext{
		get:        func() context.Context { return ctx },
		ast:        ast,
		processEnv: processEnv,
		params:     params,
		logger:     logger,
	}

	return ectx.evaluateValue(
		ns.Value,
	)
}

// EvaluateExpr compiles and executes an expr-lang expression using the AST's
// namespaces as the environment. Simple namespace names resolve as identifiers,
// parameterized namespaces are callable functions, and the env() builtin is
// available.
//
// Options are applied as overrides to ast.opts for this evaluation only.
func (ast *AST) EvaluateExpr(
	ctx context.Context,
	source string,
	opts ...Option,
) (any, error) {
	// Copy ast.opts locally for thread-safe concurrent evaluation
	local := ast.opts
	logger := ast.logger

	for _, opt := range opts {
		var temp AST

		temp.opts = local
		temp.logger = logger
		opt(&temp)
		local = temp.opts
		logger = temp.logger
	}

	processEnv := buildProcessEnvMap(local.processEnv)

	// Build compile-time environment using type exemplars from the AST.
	topScope := scope{ns: ast.Namespaces, params: nil}
	compileEnv := buildExprEnv(ctx, []scope{topScope}, processEnv, ast.logger)

	logger.TraceContext(
		ctx,
		"eval expr",
		slog.String("source", source),
		slog.Any("compile_env_keys", sortedKeys(compileEnv)),
	)

	patcher := &hyphenPatcher{
		namespaces: ast.Namespaces,
		env:        compileEnv,
		logger:     logger,
	}

	program, err := expr.Compile(
		source, expr.Env(compileEnv), expr.Patch(patcher),
	)
	if err != nil {
		// If the error is due to expr-valued namespaces in scope (unknown
		// types at compile time), fall back to lazy compilation using the
		// runtime environment where expr values are fully resolved.
		topScopes := []scope{topScope}
		if !scopesHaveExprValues(topScopes, nil) {
			return nil, ErrExprCompile.Wrap(err).
				With(slog.String("source", source))
		}

		logger.TraceContext(
			ctx,
			"eval expr deferred compile",
			slog.String("source", source),
		)

		program = nil
	}

	// Build runtime environment with actual resolved values.
	ectx := &evalContext{
		get:        func() context.Context { return ctx },
		ast:        ast,
		processEnv: processEnv,
		params:     make(map[string]*Value),
		logger:     logger,
	}
	runtimeEnv := ectx.buildRuntimeEnv()

	// If compilation was deferred, compile now with the runtime environment
	// where expr-valued namespaces have been fully resolved.
	if program == nil {
		runtimePatcher := &hyphenPatcher{
			namespaces: ast.Namespaces,
			env:        runtimeEnv,
			logger:     logger,
		}

		program, err = expr.Compile(
			source, expr.Env(runtimeEnv), expr.Patch(runtimePatcher),
		)
		if err != nil {
			return nil, ErrExprCompile.Wrap(err).
				With(slog.String("source", source))
		}
	}

	result, err := vm.Run(program, runtimeEnv)
	if err != nil {
		return nil, ErrExprEvaluate.Wrap(err).
			With(slog.String("source", source))
	}

	logger.TraceContext(
		ctx,
		"expr result",
		slog.String("result_type", resultTypeName(result)),
	)

	return result, nil
}

// evalContext holds the state for recursive evaluation.
type evalContext struct {
	get        func() context.Context // allows for cancellation and timeouts in recursive calls
	ast        *AST
	processEnv map[string]string
	params     map[string]*Value
	logger     log.Logger          // structured logger for trace-level debugging
	resolving  map[*Value]struct{} // cycle detection for resolveForEnv
}

// evaluateValue recursively evaluates a Value to its Go representation.
func (ctx *evalContext) evaluateValue(v *Value) (any, error) {
	if v == nil {
		return nil, nil
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"eval dispatch",
		slog.String("value_type", v.Type.String()),
	)

	switch v.Type {
	case TypeBoolean:
		return ctx.evaluateBoolean(v)

	case TypeNumber:
		return ctx.evaluateNumber(v)

	case TypeString:
		return ctx.evaluateString(v)

	case TypeIdentifier:
		return ctx.evaluateIdentifier(v)

	case TypeExpr:
		return ctx.evaluateExpr(v)

	case TypeTuple:
		return ctx.evaluateTuple(v)

	case TypeNamespace:
		return ctx.evaluateNamespace(v)

	default:
		return nil, ErrInvalidValueType.
			With(slog.String("type", v.Type.String()))
	}
}

// evaluateBoolean evaluates a boolean literal.
func (ctx *evalContext) evaluateBoolean(v *Value) (bool, error) {
	if v.Token == nil {
		return false, nil
	}

	s := v.Token.LiteralString()

	result, err := strconv.ParseBool(s)
	if err != nil {
		return false, ErrInvalidBoolean.Wrap(err).
			With(slog.String("value", s))
	}

	return result, nil
}

// evaluateNumber evaluates a number literal.
func (ctx *evalContext) evaluateNumber(v *Value) (any, error) {
	if v.Token == nil {
		return int64(0), nil
	}

	s := v.Token.LiteralString()

	// Try int first
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return i, nil
	}

	// Fall back to float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	return nil, ErrInvalidNumber.
		With(slog.String("value", s))
}

// evaluateString evaluates a string literal.
func (ctx *evalContext) evaluateString(v *Value) (string, error) {
	if v.Token == nil {
		return "", nil
	}

	s := v.Token.LiteralString()

	// Remove surrounding quotes if present
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') ||
			(first == '\'' && last == '\'') ||
			(first == '`' && last == '`') {
			if unquoted, err := strconv.Unquote(s); err == nil {
				return unquoted, nil
			}
		}
	}

	return s, nil
}

// evaluateIdentifier resolves an identifier reference.
func (ctx *evalContext) evaluateIdentifier(v *Value) (any, error) {
	if v.Token == nil {
		return nil, nil
	}

	name := v.Token.LiteralString()

	// Check parameters first
	if paramVal, ok := ctx.params[name]; ok {
		ctx.logger.TraceContext(
			ctx.get(),
			"resolve identifier",
			slog.String("name", name),
			slog.String("resolved_from", "params"),
		)

		// Evaluate the parameter's parsed Value
		return ctx.evaluateValue(paramVal)
	}

	// Check AST namespaces
	ns, found := ctx.ast.GetNamespace(name)
	if !found {
		return nil, ErrNotDefined.
			With(slog.String("name", name))
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"resolve identifier",
		slog.String("name", name),
		slog.String("resolved_from", "ast"),
	)

	// Evaluate the referenced namespace's value
	return ctx.evaluateValue(ns.Value)
}

// evaluateExpr runs a compiled expr program.
// If the program wasn't compiled (e.g., due to unknown parameter types),
// it compiles lazily at evaluation time with actual parameter types.
func (ctx *evalContext) evaluateExpr(v *Value) (any, error) {
	source := v.ExprSource()

	// Skip empty expressions
	if source == "" {
		return "", nil
	}

	// Build runtime environment with resolved parameter values
	env := ctx.buildRuntimeEnv()

	// If program is nil, compile lazily with actual types from env
	program := v.Program
	lazyCompile := program == nil
	ctx.logger.TraceContext(
		ctx.get(),
		"eval expr run",
		slog.String("source", source),
		slog.Bool("lazy_compile", lazyCompile),
		slog.Any("env_keys", sortedKeys(env)),
	)

	if program == nil {
		patcher := &hyphenPatcher{
			namespaces: ctx.ast.Namespaces,
			env:        env,
			logger:     ctx.logger,
		}

		var err error

		program, err = expr.Compile(
			source, expr.Env(env), expr.Patch(patcher),
		)
		if err != nil {
			return nil, ErrExprCompile.Wrap(err).
				With(slog.String("source", source))
		}
	}

	result, err := vm.Run(program, env)
	if err != nil {
		return nil, ErrExprEvaluate.Wrap(err).
			With(slog.String("source", source))
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"expr vm result",
		slog.String("result_type", resultTypeName(result)),
	)

	return result, nil
}

// evaluateTuple evaluates a tuple of values.
func (ctx *evalContext) evaluateTuple(v *Value) (any, error) {
	if v.Tuple == nil {
		return nil, nil
	}

	// Check if all elements are namespaces
	allNamespaces := true

	for _, val := range v.Tuple.Values {
		if val.Type != TypeNamespace {
			allNamespaces = false

			break
		}
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"eval tuple",
		slog.Int("element_count", len(v.Tuple.Values)),
		slog.Bool("all_namespaces", allNamespaces),
	)

	if allNamespaces && len(v.Tuple.Values) > 0 {
		// First pass: build a map of tuple-scoped namespace values
		// This allows expressions to reference sibling namespaces
		tupleScope := make(map[string]*Value)

		for _, val := range v.Tuple.Values {
			if val.Namespace != nil {
				name := val.Namespace.Identifier.LiteralString()
				// Only include non-expression values in the scope
				// to avoid circular evaluation
				if val.Namespace.Value != nil &&
					val.Namespace.Value.Type != TypeExpr {
					tupleScope[name] = val.Namespace.Value
				}
			}
		}

		ctx.logger.TraceContext(
			ctx.get(),
			"tuple scope",
			slog.Any("scope_keys", sortedKeys(tupleScope)),
		)

		// Create a child context with the tuple scope
		childCtx := &evalContext{
			get:        ctx.get,
			ast:        ctx.ast,
			processEnv: ctx.processEnv,
			params:     make(map[string]*Value),
			resolving:  ctx.resolving,
		}

		// Copy parent params
		maps.Copy(childCtx.params, ctx.params)

		// Add tuple scope to child params
		maps.Copy(childCtx.params, tupleScope)

		// Second pass: evaluate all namespaces with the enriched context
		result := make(map[string]any)

		for _, val := range v.Tuple.Values {
			if val.Namespace != nil {
				name := val.Namespace.Identifier.LiteralString()

				evaluated, err := childCtx.evaluateValue(val.Namespace.Value)
				if err != nil {
					return nil, err
				}

				result[name] = evaluated
			}
		}

		return result, nil
	}

	// Mixed tuple or all literals: return as slice
	result := make([]any, 0, len(v.Tuple.Values))

	for _, val := range v.Tuple.Values {
		evaluated, err := ctx.evaluateValue(val)
		if err != nil {
			return nil, err
		}

		result = append(result, evaluated)
	}

	return result, nil
}

// evaluateNamespace evaluates a nested namespace.
func (ctx *evalContext) evaluateNamespace(v *Value) (any, error) {
	if v.Namespace == nil {
		return nil, nil
	}

	return ctx.evaluateValue(v.Namespace.Value)
}

// buildRuntimeEnv constructs the environment for expr execution.
func (ctx *evalContext) buildRuntimeEnv() map[string]any {
	env := make(map[string]any)

	// Add parameters first (they take precedence)
	// Resolve each *Value to a Go value for the expr environment
	for name, paramVal := range ctx.params {
		env[name] = ctx.resolveForEnv(paramVal)
	}

	// Add top-level namespaces
	// For simple values, we can evaluate them directly
	// For expr values, we provide nil to avoid infinite recursion
	// For namespaces with parameters, add them as callable functions
	for _, ns := range ctx.ast.Namespaces {
		name := ns.Identifier.LiteralString()
		if _, exists := env[name]; !exists {
			if len(ns.Parameters) > 0 {
				// Add as a callable function
				env[name] = ctx.makeNamespaceFunc(ns)
			} else {
				env[name] = ctx.resolveForEnv(ns.Value)
			}
		}
	}

	// Add env function
	env["env"] = envFunc(ctx.processEnv)

	ctx.logger.TraceContext(
		ctx.get(),
		"runtime env",
		slog.Int("param_count", len(ctx.params)),
		slog.Int("namespace_count", len(ctx.ast.Namespaces)),
		slog.Int("total_keys", len(env)),
	)

	return env
}

// makeNamespaceFunc creates a callable function for a namespace with
// parameters.
// This allows parameterized namespaces to be called from within expressions.
// The returned closure creates a child evalContext that inherits the
// resolving set from the parent, preventing infinite recursion when
// evaluating cross-referencing expr-valued namespaces.
func (ctx *evalContext) makeNamespaceFunc(
	ns *Namespace,
) func(...any) (any, error) {
	return func(args ...any) (any, error) {
		ctx.logger.TraceContext(
			ctx.get(),
			"call namespace func",
			slog.String("name", ns.Identifier.LiteralString()),
			slog.Int("arg_count", len(args)),
		)

		if len(args) != len(ns.Parameters) {
			return nil, ErrArgumentCount.
				With(
					slog.String("name", ns.Identifier.LiteralString()),
					slog.Int("expected", len(ns.Parameters)),
					slog.Int("got", len(args)),
				)
		}

		// Build parameter bindings by converting args to Values
		params := make(map[string]*Value, len(args))

		for i, param := range ns.Parameters {
			if param.Token != nil {
				paramName := param.Token.LiteralString()
				params[paramName] = ctx.ast.parseArgToValue(
					ctx.get(), anyToArgString(args[i]),
				)
			}
		}

		// Create a child evalContext that inherits the resolving set,
		// preventing cycles when buildRuntimeEnv resolves expr values.
		childCtx := &evalContext{
			get:        ctx.get,
			ast:        ctx.ast,
			processEnv: ctx.processEnv,
			params:     params,
			logger:     ctx.logger,
			resolving:  ctx.resolving,
		}

		return childCtx.evaluateValue(ns.Value)
	}
}

// anyToArgString converts a Go value to a string suitable for parseArgToValue.
func anyToArgString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""

	case bool:
		return strconv.FormatBool(val)

	case int:
		return strconv.Itoa(val)

	case int64:
		return strconv.FormatInt(val, 10)

	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)

	case string:
		// Quote if it contains special characters
		if needsQuoting(val) {
			return strconv.Quote(val)
		}

		return val

	case []any:
		return formatSlice(val)

	case map[string]any:
		return formatMap(val)

	default:
		return fmt.Sprintf("%v", val)
	}
}

// resolveForEnv converts a Value to a Go value suitable for the expr
// environment.
// This avoids infinite recursion by not evaluating expressions.
func (ctx *evalContext) resolveForEnv(v *Value) any {
	if v == nil {
		return nil
	}

	switch v.Type {
	case TypeBoolean:
		if v.Token == nil {
			return false
		}

		result, _ := strconv.ParseBool(v.Token.LiteralString())

		return result

	case TypeNumber:
		if v.Token == nil {
			return int64(0)
		}

		s := v.Token.LiteralString()
		if i, err := strconv.ParseInt(s, 0, 64); err == nil {
			return i
		}

		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}

		return int64(0)

	case TypeString:
		if v.Token == nil {
			return ""
		}

		s := v.Token.LiteralString()
		if len(s) >= 2 {
			first, last := s[0], s[len(s)-1]
			if (first == '"' && last == '"') ||
				(first == '\'' && last == '\'') ||
				(first == '`' && last == '`') {
				if unquoted, err := strconv.Unquote(s); err == nil {
					return unquoted
				}
			}
		}

		return s

	case TypeIdentifier:
		// Look up the identifier
		if v.Token == nil {
			return nil
		}

		name := v.Token.LiteralString()

		// Check parameters first (now *Value)
		if paramVal, ok := ctx.params[name]; ok {
			return ctx.resolveForEnv(paramVal)
		}

		// Check namespaces (but don't recurse into exprs)
		if ns, ok := ctx.ast.GetNamespace(name); ok {
			return ctx.resolveForEnv(ns.Value)
		}

		return nil

	case TypeExpr:
		// Evaluate the expr if not already resolving it (cycle detection).
		// This allows non-cyclic expr references to be fully resolved in the
		// runtime environment, while returning nil for cycles to prevent
		// infinite recursion.
		if ctx.resolving == nil {
			ctx.resolving = make(map[*Value]struct{})
		}

		if _, cycling := ctx.resolving[v]; cycling {
			return nil
		}

		ctx.resolving[v] = struct{}{}
		defer delete(ctx.resolving, v)

		result, err := ctx.evaluateValue(v)
		if err != nil {
			ctx.logger.TraceContext(
				ctx.get(),
				"resolveForEnv expr failed",
				slog.String("source", v.ExprSource()),
				slog.String("error", err.Error()),
			)

			return nil
		}

		return result

	case TypeTuple:
		if v.Tuple == nil {
			return nil
		}

		// Check if all elements are namespaces
		allNamespaces := true

		for _, val := range v.Tuple.Values {
			if val.Type != TypeNamespace {
				allNamespaces = false

				break
			}
		}

		if allNamespaces && len(v.Tuple.Values) > 0 {
			result := make(map[string]any)

			for _, val := range v.Tuple.Values {
				if val.Namespace != nil {
					name := val.Namespace.Identifier.LiteralString()
					result[name] = ctx.resolveForEnv(val.Namespace.Value)
				}
			}

			return result
		}

		result := make([]any, 0, len(v.Tuple.Values))

		for _, val := range v.Tuple.Values {
			result = append(result, ctx.resolveForEnv(val))
		}

		return result

	case TypeNamespace:
		if v.Namespace == nil {
			return nil
		}

		return ctx.resolveForEnv(v.Namespace.Value)

	default:
		return nil
	}
}

// parseArgToValue converts a command-line argument string to a *Value.
// It uses smart inference to avoid requiring quoted strings:
//   - Booleans (true/false) → TypeBoolean
//   - Numbers (integers, floats) → TypeNumber
//   - Tuples ({ ... }) → TypeTuple
//   - Namespaces (contains :) → TypeNamespace
//   - Expressions ({{ ... }}) → TypeExpr
//   - Quoted strings ("..." or '...') → TypeString
//   - Bare words matching a namespace → TypeIdentifier
//   - Otherwise → TypeString (treated as unquoted string)
func (ast *AST) parseArgToValue(ctx context.Context, arg string) *Value {
	arg = strings.TrimSpace(arg)

	// Empty string
	if arg == "" {
		return NewString("")
	}

	// Check for boolean literals
	if arg == "true" || arg == "false" {
		return NewBool(arg == "true")
	}

	// Check for tuple (starts with {, but not {{)
	if strings.HasPrefix(arg, "{") && !strings.HasPrefix(arg, "{{") {
		if val, err := ParseValue(ctx, arg); err == nil {
			return val
		}
		// If parse fails, treat as string
		return NewString(arg)
	}

	// Check for expression (starts with {{)
	if strings.HasPrefix(arg, "{{") {
		if val, err := ParseValue(ctx, arg); err == nil {
			return val
		}

		return NewString(arg)
	}

	// Check for quoted string
	if (strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)) ||
		(strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'")) ||
		(strings.HasPrefix(arg, "`") && strings.HasSuffix(arg, "`")) {
		if val, err := ParseValue(ctx, arg); err == nil {
			return val
		}

		return NewString(arg)
	}

	// Check for namespace (contains : but not as first char)
	if idx := strings.Index(arg, ":"); idx > 0 {
		if val, err := ParseValue(ctx, arg); err == nil {
			return val
		}
		// If parse fails, treat as string
		return NewString(arg)
	}

	// Check for number (try parsing as int or float)
	if _, err := strconv.ParseInt(arg, 0, 64); err == nil {
		return NewNumber(arg)
	}

	if _, err := strconv.ParseFloat(arg, 64); err == nil {
		return NewNumber(arg)
	}

	// Check if this matches an existing namespace name (identifier reference)
	if _, found := ast.GetNamespace(arg); found {
		return NewIdentifier(arg)
	}

	// Default: treat as a plain string (no quotes needed from user)
	return NewString(arg)
}

// FormatResult formats an evaluation result for output.
// This produces an expr-lang compatible syntax representation.
func FormatResult(result any) string {
	return formatResultValue(result)
}

// formatResultValue recursively formats a Go value as aenv syntax.
func formatResultValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "nil"

	case bool:
		return strconv.FormatBool(val)

	case int:
		return strconv.Itoa(val)

	case int64:
		return strconv.FormatInt(val, 10)

	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)

	case string:
		// Always quote strings for expr-lang compatibility
		return strconv.Quote(val)

	case []any:
		return formatSlice(val)

	case map[string]any:
		return formatMap(val)

	default:
		return fmt.Sprintf("%v", val)
	}
}

// needsQuoting returns true if a string needs to be quoted.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}

	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '"' || r == '\'' || r == '\\' ||
			r == '{' || r == '}' || r == ':' || r == ',' ||
			r == '[' || r == ']' {
			return true
		}
	}

	return false
}

// formatSlice formats a slice as expr-lang array syntax.
func formatSlice(vals []any) string {
	if len(vals) == 0 {
		return "[]"
	}

	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = formatResultValue(v)
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

// formatMap formats a map as expr-lang map syntax.
func formatMap(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(m))

	for k, v := range m {
		parts = append(parts, strconv.Quote(k)+": "+formatResultValue(v))
	}

	return "{" + strings.Join(parts, ", ") + "}"
}
